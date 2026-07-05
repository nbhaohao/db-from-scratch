package db0604

import "bytes"

type MergedSortedKV []SortedKV

// 你来实现（把 k 个各自有序的 SortedKV 合并成一个按序遍历的迭代器 = merge sort 的 k 路归并。
// m 里索引小的层优先级高:MemTable 在前、SSTable 在后，同 key 以靠前的层为准）:
//  1. levels := make([]SortedKVIter, len(m))；对每个 sub := m[i] 调 sub.Iter() 存进 levels[i](出错 return nil, err)
//  2. 当前该输出哪层 = 所有 Valid 的 levels 里 Key 最小的那个下标；key 相同时取索引更小(更高优先级)的层。
//     建议抽个辅助函数 levelsLowest(levels)：遍历，用 bytes.Compare 记住 key 最小的 index，没有 Valid 的返回 -1
//  3. return &MergedSortedKVIter{levels, levelsLowest(levels)}, nil
func (m MergedSortedKV) Iter() (iter SortedKVIter, err error) {
	levels := make([]SortedKVIter, len(m))
	for i, sub := range m {
		levels[i], err = sub.Iter()
		if err != nil {
			return nil, err
		}
	}
	return &MergedSortedKVIter{levels: levels, which: levelsLowest(levels)}, nil
}

func levelsLowest(levels []SortedKVIter) int {
	which := -1
	for i, sub := range levels {
		if !sub.Valid() {
			continue
		}
		if which < 0 || bytes.Compare(sub.Key(), levels[which].Key()) < 0 {
			which = i
		}
	}
	return which
}

func levelsHighest(levels []SortedKVIter) int {
	which := -1
	for i, sub := range levels {
		if !sub.Valid() {
			continue
		}
		if which < 0 || bytes.Compare(sub.Key(), levels[which].Key()) > 0 {
			which = i
		}
	}
	return which
}

type MergedSortedKVIter struct {
	levels []SortedKVIter
	which  int
}

func (iter *MergedSortedKVIter) Valid() bool {
	return iter.which >= 0
}
func (iter *MergedSortedKVIter) Key() []byte {
	return iter.levels[iter.which].Key()
}
func (iter *MergedSortedKVIter) Val() []byte {
	return iter.levels[iter.which].Val()
}

// 你来实现（升序前进一步。难点:各层 key 范围重叠，当前这个 key 可能同时躺在多层，都得跳过，再重选最小）:
//  1. 先记下当前 key:cur := iter.Key()(若当前 !Valid 则 cur = nil)
//  2. 遍历每一层 sub:若该层 !Valid，或 bytes.Compare(cur, sub.Key()) >= 0(该层正停在 <= cur 处)，
//     就 sub.Next() 让它前进(出错 return err)——这样所有 == cur 的重复 key 一并被跳过
//  3. iter.which = levelsLowest(iter.levels) 重新选出下一个最小 key 的层
//  4. return nil
func (iter *MergedSortedKVIter) Next() error {
	var cur []byte
	if iter.Valid() {
		cur = iter.Key()
	}

	for _, sub := range iter.levels {
		if !sub.Valid() || bytes.Compare(cur, sub.Key()) >= 0 {
			if err := sub.Next(); err != nil {
				return err
			}
		}
	}
	iter.which = levelsLowest(iter.levels)
	return nil
}

// 你来实现（降序后退一步，Next 的镜像:比较符反向，改挑最大的那层）:
//  1. cur := iter.Key()(无效则 nil)
//  2. 遍历每层:若 !Valid 或 bytes.Compare(cur, sub.Key()) <= 0，就 sub.Prev()(出错 return err)
//  3. iter.which = levelsHighest(iter.levels)——在 Valid 的层里挑 Key 最大的下标(可另写一个跟 levelsLowest 对称的辅助函数)
//  4. return nil
func (iter *MergedSortedKVIter) Prev() error {
	var cur []byte
	if iter.Valid() {
		cur = iter.Key()
	}

	for _, sub := range iter.levels {
		if !sub.Valid() || bytes.Compare(cur, sub.Key()) <= 0 {
			if err := sub.Prev(); err != nil {
				return err
			}
		}
	}
	iter.which = levelsHighest(iter.levels)
	return nil
}

// UzBVUkNF https://systems-programming.org/
