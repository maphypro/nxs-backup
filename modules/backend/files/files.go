package files

import (
	"io"
	"os"
	"path/filepath"

	"github.com/juju/ratelimit"
	"gopkg.in/ini.v1"

	"github.com/nixys/nxs-backup/misc"
)

type limitedWriteCloser struct {
	w io.Writer
	c io.Closer
}

type LimitedReadCloser struct {
	r io.Reader
	c io.Closer
	s io.Seeker
}

func (lwc *limitedWriteCloser) Write(p []byte) (int, error) {
	return lwc.w.Write(p)
}

func (lwc *limitedWriteCloser) Close() error {
	return lwc.c.Close()
}

func (lrc *LimitedReadCloser) Read(p []byte) (int, error) {
	return lrc.r.Read(p)
}

func (lrc *LimitedReadCloser) Close() error {
	return lrc.c.Close()
}

func (lrc *LimitedReadCloser) Seek(offset int64, whence int) (int64, error) {
	return lrc.s.Seek(offset, whence)
}

func CreateTmpMysqlAuthFile(af *ini.File) (authFile string, err error) {
	authFile = filepath.Join("/tmp", misc.RandString(20))
	file, err := os.OpenFile(authFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	if _, err = af.WriteTo(file); err != nil {
		return
	}
	return
}

// CreateTmpMysqlDefaultsFile writes a single MySQL option file that combines the
// user-provided defaults file (e.g. the server my.cnf, needed for datadir and
// other server settings) with the connection credentials section, and returns
// its path. It is passed to xtrabackup/mariadb-backup as a single
// `--defaults-file`: these tools accept only one leading "defaults" option and
// it must come first, so credentials cannot be supplied via a separate
// `--defaults-extra-file` when `--defaults-file` is used.
func CreateTmpMysqlDefaultsFile(srcDefaultsFile string, af *ini.File) (defaultsFile string, err error) {
	src, err := os.ReadFile(srcDefaultsFile)
	if err != nil {
		return "", err
	}

	defaultsFile = filepath.Join("/tmp", misc.RandString(20))
	file, err := os.OpenFile(defaultsFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	// Inline the user's defaults file first (keeps its groups, e.g. [mysqld]
	// datadir), then append the credentials group.
	if _, err = file.Write(src); err != nil {
		return
	}
	if _, err = file.WriteString("\n"); err != nil {
		return
	}
	if _, err = af.WriteTo(file); err != nil {
		return
	}
	return
}

func DeleteTmpMysqlAuthFile(path string) error {
	return os.RemoveAll(path)
}

func GetLimitedFileWriter(filePath string, rateLim int64) (io.WriteCloser, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	lwc := &limitedWriteCloser{
		c: file,
	}
	if rateLim != 0 {
		bucket := ratelimit.NewBucketWithRate(float64(rateLim), rateLim*2)
		lwc.w = ratelimit.Writer(file, bucket)
	} else {
		lwc.w = file
	}

	return lwc, nil
}

func GetLimitedFileReader(filePath string, rateLim int64) (*LimitedReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	lrc := &LimitedReadCloser{
		c: file,
		s: file,
	}
	if rateLim != 0 {
		bucket := ratelimit.NewBucketWithRate(float64(rateLim), rateLim*2)
		lrc.r = ratelimit.Reader(file, bucket)
	} else {
		lrc.r = file
	}

	return lrc, err
}
