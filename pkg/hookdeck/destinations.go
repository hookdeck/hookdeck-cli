package hookdeck

type Destination struct {
	Id      string `json:"id"`
	Alias   string `json:"alias"`
	Label   string `json:"label"`
	CliPath string `json:"cli_path"`
}

type CreateDestinationInput struct {
	Alias   string `json:"alias"`
	Label   string `json:"label"`
	CliPath string `json:"cli_path"`
}
