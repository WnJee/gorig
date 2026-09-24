package domainx

type ConType string

const (
	Mysql ConType = "mysql"
	Mongo ConType = "mongo"
)

const defaultDBName = "main"

func (c ConType) String() string {
	return string(c)
}

func (c *Con) GetConType() ConType {
	if c == nil {
		return ""
	}
	if c.ConType == "" {
		return Mysql
	}
	return c.ConType
}

func (c *Con) GetConStr() string {
	if c == nil {
		return ""
	}
	switch c.GetConType() {
	case Mysql:
		return "mysql"
	case Mongo:
		return "mongo"
	}
	return ""
}
