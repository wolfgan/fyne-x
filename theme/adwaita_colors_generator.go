//go:build ignore
// +build ignore

/*
This tool will visit the Adwaita color page and generate a Go file with all the colors.
To add a new color, just add it to the colorToGet map. The key is the name of the color for Fyne, and the color name
is the name of the color in the Adwaita page without the "@".
*/

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"image/color"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
)

const (
	adwaitaColorPage = "https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.0/named-colors.html"
	output           = "adwaita_colors.go"
	sourceTpl        = `package theme

// This file is generated by adwaita_colors_generator.go
// Please do not edit manually, use:
// go generate ./theme/...
//
// The colors are taken from: https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.0/named-colors.html

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var adwaitaDarkScheme = map[fyne.ThemeColorName]color.Color{
{{- range $key, $value := .DarkScheme }}
    {{$key}}: {{$value.Col}}, // Adwaita color name @{{$value.AdwName}}
{{- end }}
}

var adwaitaLightScheme = map[fyne.ThemeColorName]color.Color{
{{- range $key, $value := .LightScheme }}
    {{$key}}: {{$value.Col}}, // Adwaita color name @{{$value.AdwName}}
{{- end }}
}`
)

var (
	// All colors are described in a table. Each color is a row.
	tableRowMatcher = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)

	// The color values are in a <tt> tag, the first one is the light color, the second one is the dark color.
	// The color is described in a rgba() format, or in a #RRGGBB format.
	tableColorCellMatcher = regexp.MustCompile(`(?s)<tt>((?:rgba|#).*?)</tt>`)

	// map to describe the colors to get from the Adwaita page and the name of the color in the Fyne theme
	colorToGet = map[string]string{
		"theme.ColorNameBackground":        "window_bg_color", // or "view_bg_color"
		"theme.ColorNameForeground":        "window_fg_color", // or "view_fg_color"
		"theme.ColorNameMenuBackground":    "view_bg_color",   // not sure about this one
		"theme.ColorNameSelection":         "accent_bg_color",
		"theme.ColorNameOverlayBackground": "view_bg_color",      // not sure about this one
		"theme.ColorNamePrimary":           "accent_bg_color",    // accent_color is the primary color for Adwaita
		"theme.ColorNameInputBackground":   "view_bg_color",      // or "window_bg_color"
		"theme.ColorNameButton":            "headerbar_bg_color", // it's the closer color to the button color
		"theme.ColorNameShadow":            "shade_color",
		"theme.ColorNameSuccess":           "success_bg_color",
		"theme.ColorNameWarning":           "warning_bg_color", // Adwaita doesn't have "orange_x" color for "dark"
		"theme.ColorNameError":             "error_bg_color",
	}

	// and standard color names:
	// if the key has got 2 values, the first one is the light color, the second one is the dark color
	standardColorToGet = map[string]string{
		"theme.ColorRed":    "red_3,red_4",     // based on error_bg_color
		"theme.ColorOrange": "orange_3",        // more or less the same as warning_bg_color
		"theme.ColorYellow": "yellow_3",        // more or less the same as warning_bg_color
		"theme.ColorGreen":  "green_4,green_5", // based on success_bg_color
		"theme.ColorBlue":   "blue_3",
		"theme.ColorPurple": "purple_3",
		"theme.ColorBrown":  "brown_3",
		"theme.ColorGray":   "dark_2",
	}
)

type colorInfo struct {
	Col     string // go formated color (color.RGBA{0x00, 0x00, 0x00, 0x00})
	AdwName string // Adwaita color name from the documentation without the "@"
}

func main() {

	rows := [][]string{}
	darkScheme := map[string]colorInfo{}
	lightScheme := map[string]colorInfo{}

	reps, err := http.Get(adwaitaColorPage)
	if err != nil {
		log.Fatal(err)
	}
	defer reps.Body.Close()
	htpage, err := ioutil.ReadAll(reps.Body)
	if err != nil {
		log.Fatal(err)
	}
	// find all the rows in the tables
	rows = tableRowMatcher.FindAllStringSubmatch(string(htpage), -1)

	// inline function, to get the color for a specific name and variant
	getWidgetColorFor := func(name, variant string) (col color.RGBA, err error) {
		for _, row := range rows {
			// check if the row is for "@success_color" (@ is html encoded)
			if strings.Contains(row[0], "&#64;"+name) || strings.Contains(row[0], "@"+name) {
				// the color is in the second column
				c := tableColorCellMatcher.FindAllStringSubmatch(row[0], -1)
				switch variant {
				case "light":
					col, err = stringToColor(c[0][1])
				case "dark":
					col, err = stringToColor(c[1][1])
				}
				return
			}
		}
		return
	}

	getStandardColorFor := func(name string) (col color.RGBA, err error) {
		for _, row := range rows {
			// check if the row is for "@success_color" (@ is html encoded)
			if strings.Contains(row[0], "&#64;"+name) || strings.Contains(row[0], "@"+name) {
				// the color is in the second column
				c := tableColorCellMatcher.FindAllStringSubmatch(row[0], -1)
				col, err = stringToColor(c[0][1])
				return
			}
		}
		return
	}

	for colname, color := range colorToGet {
		lcol, err := getWidgetColorFor(color, "light")
		if err != nil {
			log.Fatal(err)
		}
		dcol, err := getWidgetColorFor(color, "dark")
		if err != nil {
			log.Fatal(err)
		}
		lightScheme[colname] = colorInfo{
			Col:     fmt.Sprintf("color.RGBA{0x%02x, 0x%02x, 0x%02x, 0x%02x}", lcol.R, lcol.G, lcol.B, lcol.A),
			AdwName: color,
		}
		darkScheme[colname] = colorInfo{
			Col:     fmt.Sprintf("color.RGBA{0x%02x, 0x%02x, 0x%02x, 0x%02x}", dcol.R, dcol.G, dcol.B, dcol.A),
			AdwName: color,
		}
	}

	for colname, color := range standardColorToGet {
		lightColorName := color
		darkColorName := color
		colors := strings.Split(color, ",")
		if len(colors) == 2 {
			lightColorName = colors[0]
			darkColorName = colors[1]
		}

		lcol, err := getStandardColorFor(lightColorName)
		if err != nil {
			log.Fatal(err)
		}
		dcol, err := getStandardColorFor(darkColorName)
		if err != nil {
			log.Fatal(err)
		}
		lightScheme[colname] = colorInfo{
			Col:     fmt.Sprintf("color.RGBA{0x%02x, 0x%02x, 0x%02x, 0x%02x}", lcol.R, lcol.G, lcol.B, lcol.A),
			AdwName: lightColorName,
		}
		darkScheme[colname] = colorInfo{
			Col:     fmt.Sprintf("color.RGBA{0x%02x, 0x%02x, 0x%02x, 0x%02x}", dcol.R, dcol.G, dcol.B, dcol.A),
			AdwName: darkColorName,
		}
	}

	out, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	tpl := template.New("source")
	tpl, err = tpl.Parse(sourceTpl)
	if err != nil {
		log.Fatal(err)
	}
	// generate the source
	buffer := bytes.NewBuffer(nil)
	err = tpl.Execute(buffer, struct {
		LightScheme map[string]colorInfo
		DarkScheme  map[string]colorInfo
	}{
		LightScheme: lightScheme,
		DarkScheme:  darkScheme,
	})
	if err != nil {
		log.Fatal(err)
	}

	// format the file
	if formatted, err := format.Source(buffer.Bytes()); err != nil {
		log.Fatal(err)
	} else {
		out.Write(formatted)
	}

}

func stringToColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 9:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	default:
		// rgba(...) format
		var a float32
		_, err = fmt.Sscanf(s, "rgba(%d, %d, %d, %f)", &c.R, &c.G, &c.B, &a)
		c.A = uint8(a * 255)
	}
	return
}
