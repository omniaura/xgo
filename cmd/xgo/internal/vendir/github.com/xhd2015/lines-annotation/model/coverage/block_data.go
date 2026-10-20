package coverage

import "github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model"

// BlockData provide a general way to process block stats.
type BlockData struct {
	Block *model.Block
	Data  interface{}
}

func (c *BlockData) GetBlock() *model.Block {
	return c.Block
}
