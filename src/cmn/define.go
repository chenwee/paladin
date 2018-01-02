package cmn

import "errors"

var (
	ErrNotExist = errors.New("not exist")
	ErrFail     = errors.New("fail")
	ErrEOF      = errors.New("EOF") // 代表结束
)
