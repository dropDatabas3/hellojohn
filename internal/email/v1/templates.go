package email

import (
	"html/template"
	"os"
	"path/filepath"
	texttpl "text/template"
)

type Templates struct {
	VerifyHTML *template.Template
	VerifyTXT  *texttpl.Template
	ResetHTML  *template.Template
	ResetTXT   *texttpl.Template
}

const (
	TemplateVerify        = "verify_email"
	TemplateReset         = "reset_password"
	TemplateUserBlocked   = "user_blocked"
	TemplateUserUnblocked = "user_unblocked"
)

type VerifyVars struct {
	UserEmail string
	Tenant    string
	Link      string
	TTL       string
}

type ResetVars struct {
	UserEmail string
	Tenant    string
	Link      string
	TTL       string
}

func LoadTemplates(dir string) (*Templates, error) {
	read := func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		return string(b), err
	}
	vh, err := read("verify_email.html")
	if err != nil {
		return nil, err
	}
	vt, err := read("verify_email.txt")
	if err != nil {
		return nil, err
	}
	rh, err := read("reset_password.html")
	if err != nil {
		return nil, err
	}
	rt, err := read("reset_password.txt")
	if err != nil {
		return nil, err
	}

	vhT, err := template.New("verify_html").Parse(vh)
	if err != nil {
		return nil, err
	}
	vtT, err := texttpl.New("verify_txt").Parse(vt)
	if err != nil {
		return nil, err
	}
	rhT, err := template.New("reset_html").Parse(rh)
	if err != nil {
		return nil, err
	}
	rtT, err := texttpl.New("reset_txt").Parse(rt)
	if err != nil {
		return nil, err
	}

	return &Templates{vhT, vtT, rhT, rtT}, nil
}
