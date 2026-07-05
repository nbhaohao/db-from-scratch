package db0701

import (
	"os"
)

type KVMetaStore struct {
	slots [2]KVMetaItem
	MultiClosers
}

type KVMetaItem struct {
	FileName string
	fp       *os.File
	data     KVMetaData
}

type KVMetaData struct {
	Version uint64
	SSTable string
}

func (meta *KVMetaStore) Open() error {
	for i := range meta.slots {
		fp, data, err := openMetafile(meta.slots[i].FileName)
		if err != nil {
			_ = meta.Close()
			return err
		}
		meta.slots[i].fp, meta.slots[i].data = fp, data
		meta.MultiClosers = append(meta.MultiClosers, fp)
	}
	return nil
}

func openMetafile(filename string) (fp *os.File, data KVMetaData, err error) {
	if fp, err = createFileSync(filename); err != nil {
		return nil, KVMetaData{}, err
	}
	if data, err = readMetaFile(fp); err != nil {
		_ = fp.Close()
		return nil, KVMetaData{}, err
	}
	return fp, data, nil
}

// 你来实现（把一个 meta 文件的字节反序列化成 KVMetaData——double buffering 的「读」一半。
// 文件格式 [crc32(4B) | size(4B) | JSON 数据(size B)]。坏数据要当成「空」返回、而不是报错，
// Open() 才能忽略反序列化失败/checksum 不符的槽，靠另一个好槽恢复）:
//  1. io.ReadAll 读出全部字节 b；只有真正的读 IO 错误(err!=nil)才 return err
//  2. len(b)<=8（连头都没写完）→ return 空 KVMetaData, nil（当空槽，不是错误）
//  3. sum=b[0:4]、size=b[4:8]；若 b 装不下 8+size → 同样当空槽返回
//  4. crc32.ChecksumIEEE 重算 b[4:8+size]，与 sum 不符 → 当空槽返回（checksum 覆盖 size+JSON）
//  5. json.Unmarshal(b[8:8+size]) 解出 data；解析失败也当空槽；成功 return data, nil
func readMetaFile(fp *os.File) (data KVMetaData, err error)

// 你来实现（把 KVMetaData 序列化写进 meta 文件——double buffering 的「写」一半。写完 fsync，
// 配合 Set() 只覆盖版本较小的那个槽，断电后坏槽被识别、好槽还在，做到原子更新）:
//  1. json.Marshal(data) 得到 JSON 字节；序列化失败视为程序 bug，用 check() 而非 return err
//  2. 前面留出 8 字节空头：slices.Concat(make([]byte, 8), b)
//  3. 先把 JSON 长度(len(b)-8)写进 b[4:8]，再把 crc32.ChecksumIEEE(b[4:]) 写进 b[0:4]（顺序不能反：先填 size 再算 crc）
//  4. fp.WriteAt(b, 0) 覆盖写到文件开头；透传错误
//  5. fp.Sync() 落盘并 return 它的结果
func writeMetaFile(fp *os.File, data KVMetaData) error

func (meta *KVMetaStore) current() int {
	if meta.slots[0].data.Version > meta.slots[1].data.Version {
		return 0
	} else {
		return 1
	}
}

func (meta *KVMetaStore) Get() KVMetaData {
	return meta.slots[meta.current()].data
}

func (meta *KVMetaStore) Set(data KVMetaData) error {
	cur := meta.current()
	if err := writeMetaFile(meta.slots[1-cur].fp, data); err != nil {
		return err
	}
	meta.slots[1-cur].data = data
	return nil
}

// UzBVUkNF https://systems-programming.org/
