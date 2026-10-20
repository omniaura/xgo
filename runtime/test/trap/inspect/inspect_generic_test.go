package inspect

func GenericFunc[T any]() {
}

type GenericType[T any] struct {
}

func (g *GenericType[T]) GenericMethod() {
}
