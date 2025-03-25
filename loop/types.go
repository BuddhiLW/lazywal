package loop

type Position struct {
	X float32
	Y float32
}

type Monitor struct {
	Name       string
	Dimensions Size
	Position   Position
	Primary    bool
}
