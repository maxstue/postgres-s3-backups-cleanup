package backup

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"io"
	"log/slog"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/config"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/logger"

	"github.com/klauspost/pgzip"
	"github.com/miolini/datacounter"
)

func Run(ctx context.Context) error {
	logger.Stdout.Info("Streaming database backup to S3...")
	sTime := time.Now()

	dump, upload, err := streamBackup(ctx, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "pg_dump",
			"-d", config.Backup.DatabaseURL,
			"-F", "custom",
			"-Z", "0",
		)
	}, UploadToS3)
	if err != nil {
		return err
	}

	dumpedLog := logger.Stdout.With(
		slog.String("filename", dump.filename),
		logger.SizeAttrIEC(dump.compressedSize, "compressed_filesize"),
		logger.SizeAttrIEC(dump.uncompressedSize, "un_compressed_filesize"),
		slog.String("duration", dump.funcTime.Round(time.Millisecond).String()),
	)

	if dump.stderr != "" {
		dumpedLog.Warn("Database dumped with warnings",
			slog.String("stderr", dump.stderr),
		)
	} else {
		dumpedLog.Info("Database dumped")
	}

	logger.Stdout.Info("Backup Uploaded",
		slog.String("filename_in_bucket", dump.filename),
		slog.String("duration", upload.funcTime.Round(time.Millisecond).String()),
		logger.SizeAttrIEC(dump.compressedSize, "uploaded_size"),
	)

	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	logger.Stdout.Info("Database backup complete",
		slog.String("duration", time.Since(sTime).Round(time.Millisecond).String()),
		slog.Group("mem_stats",
			logger.SizeAttrIEC(memStats.TotalAlloc, "total_alloc"),
			logger.SizeAttrIEC(memStats.Sys, "sys"),
		),
	)

	return nil
}

type dumpInfo struct {
	basename         string
	filename         string
	compressedSize   uint64
	uncompressedSize uint64
	stderr           string
	funcTime         time.Duration
}

// streamBackup keeps only compression blocks and S3 upload parts in memory.
// The pipe applies backpressure and only signals EOF after pg_dump and gzip
// both succeed, so a failed dump cannot be committed as a successful upload.
func streamBackup(ctx context.Context, newCommand func(context.Context) *exec.Cmd,
	upload func(context.Context, string, io.ReadCloser) (*uploadInfo, error),
) (*dumpInfo, *uploadInfo, error) {
	if config.Backup.GzipConcurrency <= 0 {
		return nil, nil, errors.New("GZIP_CONCURRENCY must be greater than zero")
	}
	if config.Backup.UploadConcurrency <= 0 {
		return nil, nil, errors.New("UPLOAD_CONCURRENCY must be greater than zero")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, context.Cause(ctx)
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	// Context cancellation alone cannot interrupt a blocked pipe read or write.
	stopWatching := context.AfterFunc(ctx, func() {
		reader.CloseWithError(context.Cause(ctx))
		writer.CloseWithError(context.Cause(ctx))
	})
	defer stopWatching()

	dump := &dumpInfo{}
	timeNowUTC := time.Now().UTC()
	timestamp := timeNowUTC.Format("2006-01-02T15-04-05Z")
	dump.basename = fmt.Sprintf("backup-%s.dump", timestamp)
	dump.filename = fmt.Sprintf("%s.gz", dump.basename)
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation.name", "pg_dump"),
	}
	if databaseURL, err := url.Parse(config.Backup.DatabaseURL); err == nil {
		if host := databaseURL.Hostname(); host != "" {
			attrs = append(attrs, attribute.String("server.address", host))
		}
		if database := strings.TrimPrefix(databaseURL.Path, "/"); database != "" {
			attrs = append(attrs, attribute.String("db.namespace", database))
		}
	}
	dumpCtx, dumpSpan := otel.Tracer("postgres-s3-backups").Start(ctx, "pg_dump",
		trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
	cmd := newCommand(dumpCtx)
	// Bound the wait if a child inherits stdout/stderr and keeps them open.
	cmd.WaitDelay = 5 * time.Second

	dumpResult := make(chan error, 1)
	go func() {
		err := dumpToStream(cmd, writer, dump, timeNowUTC, cancel)
		if err != nil {
			dumpSpan.SetStatus(codes.Error, "")
		}
		dumpSpan.End()
		if err != nil {
			writer.CloseWithError(err)
			cancel(err)
		} else {
			writer.Close()
		}
		dumpResult <- err
	}()

	info, uploadErr := upload(ctx, dump.filename, reader)
	if uploadErr != nil {
		cancel(uploadErr)
		reader.CloseWithError(uploadErr)
	}

	// Always reap pg_dump and finish compression before returning.
	dumpErr := <-dumpResult
	err := errors.Join(dumpErr, uploadErr)
	if cause := context.Cause(ctx); cause != nil && !errors.Is(err, cause) {
		err = errors.Join(err, cause)
	}
	if err != nil {
		return nil, nil, err
	}
	return dump, info, nil
}

func dumpToStream(cmd *exec.Cmd, writer *io.PipeWriter, dump *dumpInfo, modTime time.Time, cancel context.CancelCauseFunc) (err error) {
	output := &compressionOutput{writer: writer, cancel: cancel}
	compressed := datacounter.NewWriterCounter(output)
	compressor := pgzip.NewWriter(compressed)
	if err := compressor.SetConcurrency(5*1024*1024, config.Backup.GzipConcurrency); err != nil {
		return fmt.Errorf("configure gzip compression: %w", err)
	}
	compressor.Header = pgzip.Header{Name: dump.basename, ModTime: modTime}
	uncompressed := datacounter.NewWriterCounter(compressor)
	stderr := &stderrTail{}
	cmd.Stdout = uncompressed
	cmd.Stderr = stderr

	startTime := time.Now()
	defer func() {
		if err != nil {
			// Unblock gzip before closing it and never turn a failed dump into EOF.
			writer.CloseWithError(err)
		}
		// Close flushes pending compression blocks and writes the CRC/size
		// trailer. Finish it before the caller closes the pipe and signals EOF.
		if closeErr := compressor.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("finish gzip stream: %w", closeErr))
		}
		if writeErr := output.Err(); writeErr != nil {
			err = errors.Join(err, fmt.Errorf("write compressed backup stream: %w", writeErr))
		}
		dump.funcTime = time.Since(startTime)
		dump.compressedSize = compressed.Count()
		dump.uncompressedSize = uncompressed.Count()
		dump.stderr = strings.TrimSpace(stderr.String())
	}()

	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("run pg_dump: %w", err)
		if diagnostic := strings.TrimSpace(stderr.String()); diagnostic != "" {
			err = errors.Join(err, getPgDumpError(diagnostic))
		}
		return err
	}
	if uncompressed.Count() == 0 {
		return fmt.Errorf("pg_dump wrote 0 bytes: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// pgzip can leave its output goroutine waiting forever if its destination
// returns an error. Record pipe failures and cancel pg_dump immediately, but
// let gzip drain and close normally. The recorded error still fails the backup.
type compressionOutput struct {
	writer io.Writer
	cancel context.CancelCauseFunc
	mu     sync.Mutex
	err    error
}

func (w *compressionOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	failed := w.err != nil
	w.mu.Unlock()
	if !failed {
		if _, err := w.writer.Write(p); err != nil {
			w.mu.Lock()
			w.err = err
			w.mu.Unlock()
			w.cancel(err)
		}
	}
	return len(p), nil
}

func (w *compressionOutput) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

const maxStderrBytes = 64 * 1024

// Keep the end of stderr, where pg_dump normally reports its final error.
// Always consume the entire write so verbose diagnostics cannot block the dump.
type stderrTail struct {
	data      []byte
	truncated bool
}

func (b *stderrTail) Write(p []byte) (int, error) {
	n := len(p)
	if len(b.data)+n > maxStderrBytes {
		b.truncated = true
		if n >= maxStderrBytes {
			b.data = append(b.data[:0], p[n-maxStderrBytes:]...)
			return n, nil
		}
		keep := maxStderrBytes - n
		copy(b.data, b.data[len(b.data)-keep:])
		b.data = b.data[:keep]
	}
	b.data = append(b.data, p...)
	return n, nil
}

func (b *stderrTail) String() string {
	if b.truncated {
		return "[earlier stderr truncated]\n" + string(b.data)
	}
	return string(b.data)
}

// looks for error text and returns it, stripping away any potential prefixed warnings.
//
// if no error text is found, the original string is returned.
func getPgDumpError(stderr string) error {
	const errorSearchText = "pg_dump: error:"

	if index := strings.Index(stderr, errorSearchText); index != -1 {
		return errors.New(stderr[index:])
	}

	return errors.New(stderr)
}
