package uploadgate

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var webDistEmbed embed.FS

//go:embed all:widget/dist
var widgetDistEmbed embed.FS

// GetWebFS returns a sub-filesystem rooted at web/dist.
func GetWebFS() fs.FS {
	sub, err := fs.Sub(webDistEmbed, "web/dist")
	if err != nil {
		return webDistEmbed
	}
	return sub
}

// GetWidgetFS returns a sub-filesystem rooted at widget/dist.
func GetWidgetFS() fs.FS {
	sub, err := fs.Sub(widgetDistEmbed, "widget/dist")
	if err != nil {
		return widgetDistEmbed
	}
	return sub
}
