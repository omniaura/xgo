package func_list

func init() {
	addExtraPkgsAssert = func(m map[string]bool) {
		m[testPkgPath+"."+"generic"] = true
		m[testPkgPath+"."+"List.size"] = true
	}
}

func generic[T any]() {
}

type List[T any] struct {
}

func (c *List[T]) size() int {
	return 0
}
