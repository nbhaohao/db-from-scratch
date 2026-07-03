package db0105

import (
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

func (log *Log) Write(ent *Entry) error {
	if _, err := log.fp.Write(ent.Encode()); err != nil {
		return err
	}
	return log.fp.Sync() // fsync
}

// 你来实现（0103 版 Read 之上，把「半条记录」也当成正常结尾忽略掉）：
//  1. err = ent.Decode(log.fp)
//  2. 三种错误都算读到头了（eof=true, err=nil）：io.EOF（正常尾）、
//     io.ErrUnexpectedEOF（文件大小不对/截断）、ErrBadSum（校验失败=torn write）
//  3. 其他 err 照常返回；无错则 eof=false
//     受损的只可能是最后一条（之前的都 fsync 成功过），忽略它 = 崩溃后恢复到上一个一致状态
func (log *Log) Read(ent *Entry) (eof bool, err error)

// UzBVUkNF https://systems-programming.org/
