package base

import "fmt"

// BlockError 表示请求被哨兵拦截。
type BlockError struct {
	blockType     BlockType    // 阻塞类型，rpc,db...
	blockMsg      string       // 提供关于块错误的附加消息。
	rule          SentinelRule //
	snapshotValue interface{}  // 表示触发的“快照”值
}

type BlockErrorOption func(*BlockError)

func WithBlockType(blockType BlockType) BlockErrorOption {
	return func(b *BlockError) {
		b.blockType = blockType
	}
}

func WithBlockMsg(blockMsg string) BlockErrorOption {
	return func(b *BlockError) {
		b.blockMsg = blockMsg
	}
}

func WithRule(rule SentinelRule) BlockErrorOption {
	return func(b *BlockError) {
		b.rule = rule
	}
}

func WithSnapshotValue(snapshotValue interface{}) BlockErrorOption {
	return func(b *BlockError) {
		b.snapshotValue = snapshotValue
	}
}

func NewBlockError(opts ...BlockErrorOption) *BlockError {
	b := &BlockError{
		blockType: BlockTypeUnknown,
	}

	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (e *BlockError) ResetBlockError(opts ...BlockErrorOption) {
	for _, opt := range opts {
		opt(e)
	}
	return
}

func (e *BlockError) BlockMsg() string {
	return e.blockMsg
}

func (e *BlockError) BlockType() BlockType {
	return e.blockType
}

func (e *BlockError) TriggeredRule() SentinelRule {
	return e.rule
}

func (e *BlockError) TriggeredValue() interface{} {
	return e.snapshotValue
}

func NewBlockErrorFromDeepCopy(from *BlockError) *BlockError {
	return &BlockError{
		blockType:     from.blockType,
		blockMsg:      from.blockMsg,
		rule:          from.rule,
		snapshotValue: from.snapshotValue,
	}
}

func NewBlockErrorWithMessage(blockType BlockType, message string) *BlockError {
	return NewBlockError(WithBlockType(blockType), WithBlockMsg(message))
}

func NewBlockErrorWithCause(blockType BlockType, blockMsg string, rule SentinelRule, snapshot interface{}) *BlockError {
	return NewBlockError(WithBlockType(blockType), WithBlockMsg(blockMsg), WithRule(rule), WithSnapshotValue(snapshot))
}

func (e *BlockError) Error() string {
	if e == nil {
		return "nil *BlockError"
	}

	if len(e.blockMsg) == 0 {
		return fmt.Sprintf("SentinelBlockError: %s", e.blockType.String())
	}
	return fmt.Sprintf("SentinelBlockError: %s, message: %s", e.blockType.String(), e.blockMsg)
}
