package vo

type BuildVO struct {
	ID      int    `json:"id"`
	Product int    `json:"product"`
	Project int    `json:"project"`
	Name    string `json:"name"`
	Date    string `json:"date"`
}
