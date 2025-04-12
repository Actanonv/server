package mux

import "github.com/mayowa/templates"

var templateMgr *templates.Template

func InitTemplates(rootFolder string, options templates.TemplateOptions) (*templates.Template, error) {

	if templateMgr != nil {
		return templateMgr, nil
	}

	templates, err := templates.New(rootFolder, &options)
	if err != nil {
		return nil, err
	}

	templateMgr = templates
	return templates, nil
}
