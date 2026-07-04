package db0104

import (
	"io"
	"os"
)

type Log struct {
	FileName string
	fp       *os.File
}

func (log *Log) Open() (err error) {
	log.fp, err = createFileSync(log.FileName)
	return err
}

func (log *Log) Close() error {
	return log.fp.Close()
}

// 你来实现（本关只加一件事：写完 fsync 落盘）：
//  1. log.fp.Write(ent.Encode())，出错返回
//  2. return log.fp.Sync()  // Sync() 就是 fsync 系统调用
//     普通 Write 只到 OS 的 page cache，断电就丢；Sync() 强制刷到硬盘并等待完成 = durability
func (log *Log) Write(ent *Entry) error {
	if _, err := log.fp.Write(ent.Encode()); err != nil {
		return err
	}
	return log.fp.Sync()
}

func (log *Log) Read(ent *Entry) (eof bool, err error) {
	err = ent.Decode(log.fp)
	if err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	} else {
		return false, nil
	}
}

// UzBVUkNF https://systems-programming.org/
