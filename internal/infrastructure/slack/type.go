package slack

type Result struct {
	Ok          bool       `json:"ok"`
	Error       string     `json:"error"`
	AccessToken string     `json:"access_token"`
	AuthedUser  AuthedUser `json:"authed_user"`
}
type AuthedUser struct {
	AccessToken string `json:"access_token"`
}
