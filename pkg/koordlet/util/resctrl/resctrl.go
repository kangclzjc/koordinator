package resctrl

type Resctrl struct {
	L3 map[int64]int64
	MB map[int64]int64
}

type App struct {
	Resctrl Resctrl
	//Hooks   Hook
	Closid string
}

type ResctrlEngine interface {
	Rebuild() // rebuild the current control group
	GetCurrentCtrlGroups() map[string]Resctrl
	Config(config string)
	GetConfig() map[string]string
	RegisterApp(podid, annotation, closid string) error
	GetApp(podid string) (App, error)
}

type RDTEngine struct {
	Apps       map[string]App
	CtrlGroups map[string]Resctrl
}

func (R RDTEngine) Rebuild() {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) GetCurrentCtrlGroups() map[string]Resctrl {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) Config(config string) {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) GetConfig() map[string]string {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) RegisterApp(podid, annotation, closid string) error {
	//TODO implement me
	panic("implement me")
}

func (R RDTEngine) GetApp(podid string) (App, error) {
	//TODO implement me
	panic("implement me")
}

func NewRDTEngine() RDTEngine {
	return RDTEngine{}
}
