// Package mini provides utilities for parsing and manipulating Minecraft text colors and styles.
// It includes functions for parsing strings with embedded style information, modifying styles,
// and creating gradient effects. It also provides functions for parsing color names and hex codes,
// and for linear interpolation of colors.
//
// Credits to the partial Go port of MiniMessage (https://docs.advntr.dev/minimessage/index.html) by
// https://github.com/emortalmc/GateProxy/blob/main/minimessage/minimessage.go.
// Also creds to https://github.com/minekube/gate-plugin-template/blob/main/util/mini/mini.go
package minimessage

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	keyCommon "go.minekube.com/common/minecraft/key"
	"go.minekube.com/common/minecraft/nbt"
)

// Parse takes a string as input and returns a `c.Text` object. It splits the input string by "<",
// then further splits each substring by ">". It modifies the style based on the key (the part before ">")
// and appends a new text component with the modified style and content (the part after ">").
func Parse(mini string) *c.Text {
	var styles []c.Style
	styles = append(styles, c.Style{Color: color.White})

	var components []c.Component

	for _, s := range strings.Split(mini, "<") {
		if s == "" {
			continue
		}

		split := strings.Split(s, ">")

		key := split[0]
		if strings.HasPrefix(key, "/") {
			styles = styles[:len(styles)-1]
		} else {
			newStyle := styles[len(styles)-1]

			styles = append(styles, newStyle)
		}

		newText := modify(key, split[1], &styles[len(styles)-1])
		components = append(components, newText)

	}

	return &c.Text{
		Extra: components,
	}
}

// modify takes a key, content, and style as input and returns a `c.Text` object. It modifies the style
// based on the key and returns a new text component with the modified style and content.
func modify(key string, content string, style *c.Style) *c.Text {
	newText := &c.Text{}

	switch {
	case strings.HasPrefix(key, "#"): // <#ff00ff>
		parsed, err := parseColor(key)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		style.Color = parsed
		newText.Content = content
		newText.S = *style
	case strings.HasPrefix(key, "color"): // <color:light_purple>
		colorName := strings.Split(key, ":")[1]
		parsed, err := parseColor(colorName)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		style.Color = parsed
		newText.Content = content
		newText.S = *style

	case key == "bold" || key == "b": // <bold>
		style.Bold = c.True
		newText.Content = content
		newText.S = *style

	case key == "italic" || key == "em" || key == "i": // <italic>
		style.Italic = c.True
		newText.Content = content
		newText.S = *style

	case key == "underlined" || key == "u": // <underlined>
		style.Underlined = c.True
		newText.Content = content
		newText.S = *style

	case key == "strikethrough" || key == "st": // <strikethrough>
		style.Strikethrough = c.True
		newText.Content = content
		newText.S = *style

	case key == "obfuscated" || key == "obf": // <obfuscated>
		style.Obfuscated = c.True
		newText.Content = content
		newText.S = *style

	case strings.HasPrefix(key, "click"): // <click:run_command:/seed>
		clickKey := strings.Split(key, ":")
		clickAction := clickKey[1]
		clickValue := clickKey[2]
		switch clickAction {
		case "change_page":
			style.ClickEvent = c.ChangePage(clickValue)
		case "copy_to_clipboard":
			style.ClickEvent = c.CopyToClipboard(clickValue)
		case "open_file":
			style.ClickEvent = c.OpenFile(clickValue)
		case "open_url":
			style.ClickEvent = c.OpenUrl(clickValue)
		case "run_command":
			style.ClickEvent = c.RunCommand(clickValue)
		case "suggest_command":
			style.ClickEvent = c.SuggestCommand(clickValue)
		}
		newText.Content = content
		newText.S = *style

	case strings.HasPrefix(key, "hover"): // <hover:show_text:test>
		hoverKey := strings.Split(key, ":")
		hoverAction := hoverKey[1]
		hoverValue := hoverKey[2]
		switch hoverAction {
		case "show_text":
			// TODO: parse using Parse()
			style.HoverEvent = c.ShowText(&c.Text{
				Content: hoverValue,
			}) // _text_
		case "show_item":
			showItemKeys := strings.Split(hoverValue, ":")
			itemType, _ := keyCommon.Parse(showItemKeys[0])
			itemCount := 0 // not sure whats the default,
			itemTag := nbt.NewBinaryTagHolder("")
			if len(showItemKeys) >= 2 {
				count, _ := strconv.Atoi(showItemKeys[1])
				itemCount = count
				if len(showItemKeys) == 3 {
					itemTag = nbt.NewBinaryTagHolder(showItemKeys[2])
				}
			}
			style.HoverEvent = c.ShowItem(&c.ShowItemHoverType{
				Item:  itemType,
				Count: itemCount,
				NBT:   itemTag,
			}) // _type_[:_count_[:tag]]
		case "show_entity":
			showEntityKeys := strings.Split(hoverValue, ":")
			entityType, _ := keyCommon.Parse(showEntityKeys[0])
			entityId, _ := uuid.Parse(showEntityKeys[1])
			entityName := &c.Text{}
			if len(showEntityKeys) == 3 {
				entityName = Parse(showEntityKeys[2])
			}
			style.HoverEvent = c.ShowEntity(&c.ShowEntityHoverType{
				Type: entityType,
				Id:   entityId,
				Name: entityName,
			}) // _type_:_uuid_[:_name_]
		}
		newText.Content = content
		newText.S = *style

	case strings.HasPrefix(key, "gradient"): // <gradient:light_purple:gold>
		colorKey := strings.Split(key, ":")
		colorNames := colorKey[1:]

		colors := make([]color.RGB, len(colorNames))
		for i, col := range colorNames {
			parsedColor, err := parseColor(col)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			newColor, _ := color.Make(parsedColor)
			colors[i] = *newColor
		}

		newText = gradient(content, *style, colors...)
	}

	return newText
}

// parseColor takes a string as input and returns a `color.Color` object. It checks if the input string
// starts with "#". If it does, it tries to parse it as a hex color. If it doesn't, it tries to find a
// named color that matches the input string.
func parseColor(name string) (color.Color, error) {
	if strings.HasPrefix(name, "#") {
		newColor, err := color.Hex(name)
		if err != nil {
			return nil, err
		}
		return newColor, nil
	} else {
		return fromName(name)
	}
}

// fromName takes a string as input and returns a `color.Color` object.
// It iterates over the named colors and returns the one that matches the input string.
func fromName(name string) (color.Color, error) {
	col, ok := color.Names[name]
	if ok {
		return col, nil
	}
	for _, a := range color.Names {
		if strings.EqualFold(a.String(), name) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("unknown color name: %s", name)
}

// gradient takes a string, a style, and a variable number of colors as input and returns a `c.Text` object.
// It creates a gradient effect by interpolating between the input colors based on their position in the input string.
func gradient(content string, style c.Style, colors ...color.RGB) *c.Text {
	var component []c.Component
	for id, i := range strings.Split(content, "") {
		t := float64(id) / float64(len(content))
		hex, _ := color.Hex(lerpColor(t, colors...).Hex())

		style.Color = hex
		component = append(component, &c.Text{
			Content: string(i),
			S:       style,
		})
	}

	return &c.Text{
		Extra: component,
	}
}

// lerpColor takes a float and a variable number of colors as input and returns a `color.Color` object.
// It interpolates between the input colors based on the input float.
func lerpColor(t float64, colors ...color.RGB) color.Color {
	t = math.Min(t, 1)

	if t == 1 {
		return &colors[len(colors)-1]
	}

	colorT := t * float64(len(colors)-1)
	newT := colorT - math.Floor(colorT)
	lastColor := colors[int(colorT)]
	nextColor := colors[int(colorT+1)]

	return &color.RGB{
		R: lerpInt(newT, nextColor.R, lastColor.R),
		G: lerpInt(newT, nextColor.G, lastColor.G),
		B: lerpInt(newT, nextColor.B, lastColor.B),
	}
}

// lerpInt takes three floats as input and returns a float. It performs linear interpolation between the
// second and third input floats based on the first input float.
func lerpInt(t float64, a float64, b float64) float64 {
	return a*t + b*(1-t)
}
