package db0805

import (
	"errors"
	"io"
	"os"
)

type Log struct {
	FileName string
	fp       *os.File
	reader   OffsetReader
	writer   struct {
		offset    int64
		committed int64
	}
}

func (log *Log) Open() (err error) {
	log.fp, err = createFileSync(log.FileName)
	log.reader = OffsetReader{log.fp, 0}
	log.writer.offset = 0
	log.writer.committed = 0
	return err
}

func (log *Log) Close() error {
	return log.fp.Close()
}

func (log *Log) Write(ent *Entry) error {
	data := ent.Encode()
	if _, err := log.fp.WriteAt(data, log.writer.offset); err != nil {
		return err
	}
	log.writer.offset += int64(len(data))
	return nil
}

func (log *Log) Commit() error {
	if err := log.Write(&Entry{op: EntryCommit}); err != nil {
		return err
	}
	if err := log.fp.Sync(); err != nil {
		return err
	}
	log.writer.committed = log.writer.offset
	return nil
}

func (log *Log) ResetTX() {
	log.writer.offset = log.writer.committed
}

// 你来实现（启动时读一条 log 记录并维护 offset/committed。事务原子性的「读」半边:只有读到
// EntryCommit 标记,才认为它之前的记录都已提交、可生效——半个事务(没写完 commit 就断电)会被丢弃）:
//  1. ent.Decode(&log.reader) 解一条;若 err 是 io.EOF/io.ErrUnexpectedEOF/ErrBadSum(到文件尾或半条坏记录)→ return true(eof),nil
//  2. 其它 err → return false,err 透传
//  3. 正常读到:若 ent.op==EntryCommit,把 log.writer.offset 和 log.writer.committed 都推进到 log.reader.offset(标记「已提交到这里」)
//  4. return false,nil
func (log *Log) Read(ent *Entry) (eof bool, err error) {
	err = ent.Decode(&log.reader)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || err == ErrBadSum {
			return true, nil
		}
		return false, err
	}
	if ent.op == EntryCommit {
		log.writer.offset = log.reader.offset
		log.writer.committed = log.reader.offset
	}
	return false, nil
}

func (log *Log) Truncate() error {
	log.writer.offset = 0
	log.writer.committed = 0
	return log.fp.Truncate(0)
}

type OffsetReader struct {
	inner  io.ReaderAt
	offset int64
}

func (rd *OffsetReader) Read(buf []byte) (n int, err error) {
	n, err = rd.inner.ReadAt(buf, rd.offset)
	if n > 0 {
		err = nil
	}
	rd.offset += int64(n)
	return n, err
}

// UzBVUkNF https://systems-programming.org/
