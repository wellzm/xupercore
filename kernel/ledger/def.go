// 账本约束数据结构定义
package ledger

type XMSnapshotReader interface {
	Get(bucket string, key []byte) ([]byte, error)
}

type XMReader interface {
	//读取一个key的值，返回的value就是有版本的data
	Get(bucket string, key []byte) (*VersionedData, error)
	//扫描一个bucket中所有的kv, 调用者可以设置key区间[startKey, endKey)
	Select(bucket string, startKey []byte, endKey []byte) (XMIterator, error)
}

// XMIterator iterates over key/value pairs in key order
type XMIterator interface {
	Key() []byte
	Value() *VersionedData
	Next() bool
	Error() error
	// Iterator 必须在使用完毕后关闭
	Close()
}

type PureData struct {
	Bucket string
	Key    []byte
	Value  []byte
}

type VersionedData struct {
	PureData  *PureData
	RefTxid   []byte
	RefOffset int32
}

// 区块基础操作
type BlockHandle interface {
	GetProposer() []byte
	GetHeight() int64
	GetBlockid() []byte
	GetConsensusStorage() ([]byte, error)
	GetTimestamp() int64
	SetItem(item string, value interface{}) error
	MakeBlockId() ([]byte, error)
	GetPreHash() []byte
	GetPublicKey() string
	GetSign() []byte
}
