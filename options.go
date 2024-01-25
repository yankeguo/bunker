package bunker

type DataDir string

func (d DataDir) String() string {
	return string(d)
}
