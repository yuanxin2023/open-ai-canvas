package assets

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const SVGMIME = "image/svg+xml"
const MaxSVGBytes = 16 << 20

var staticSVGElements = strings.Fields("svg g defs symbol use path rect circle ellipse line polyline polygon text tspan textPath linearGradient radialGradient stop clipPath mask pattern marker title desc style filter feBlend feColorMatrix feComponentTransfer feComposite feConvolveMatrix feDiffuseLighting feDisplacementMap feDistantLight feDropShadow feFlood feFuncA feFuncB feFuncG feFuncR feGaussianBlur feMerge feMergeNode feMorphology feOffset fePointLight feSpecularLighting feSpotLight feTile feTurbulence")
var svgLocalURL = regexp.MustCompile(`(?i)url\(\s*["']?#[a-zA-Z_][a-zA-Z0-9_.:-]*["']?\s*\)`)

// InspectStaticSVG validates generated vectors without rewriting their paths.
// This does not grant SVG inline delivery; the attachment/sandbox policy remains.
func InspectStaticSVG(data []byte) (int, int, error) {
	invalid := func() (int, int, error) {
		return 0, 0, fmt.Errorf("SVG 内容校验失败：仅支持带有效尺寸的静态矢量 XML，不支持脚本、外部资源或内嵌位图")
	}
	if len(data) == 0 || len(data) > MaxSVGBytes {
		return invalid()
	}
	decoder := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	depth, roots, width, height := 0, 0, 0, 0
	styleDepth := 0
	var css strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return invalid()
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || t.Name.Local != "svg" || t.Name.Space != "http://www.w3.org/2000/svg" {
					return invalid()
				}
				width, height = svgDimensions(t.Attr)
			}
			if t.Name.Space != "http://www.w3.org/2000/svg" || !staticSVGElement(t.Name.Local) || depth >= 128 || styleDepth > 0 {
				return invalid()
			}
			for _, attr := range t.Attr {
				name := strings.ToLower(attr.Name.Local)
				if strings.HasPrefix(name, "on") || name == "base" || !staticSVGValue(attr.Value) {
					return invalid()
				}
				if name == "href" && !strings.HasPrefix(strings.TrimSpace(attr.Value), "#") {
					return invalid()
				}
			}
			depth++
			if t.Name.Local == "style" {
				styleDepth = depth
				css.Reset()
			}
		case xml.EndElement:
			if styleDepth == depth {
				if !staticSVGValue(css.String()) {
					return invalid()
				}
				styleDepth = 0
			}
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return invalid()
			}
			if styleDepth > 0 {
				css.Write(t)
			}
		case xml.Directive:
			return invalid()
		case xml.ProcInst:
			if t.Target != "xml" || roots != 0 {
				return invalid()
			}
		}
	}
	if roots != 1 || depth != 0 || width <= 0 || height <= 0 {
		return invalid()
	}
	return width, height, nil
}

func staticSVGElement(name string) bool {
	for _, allowed := range staticSVGElements {
		if name == allowed {
			return true
		}
	}
	return false
}

func staticSVGValue(value string) bool {
	value = strings.ToLower(svgLocalURL.ReplaceAllString(value, ""))
	for _, forbidden := range []string{"url", "@", "\\", "/*", "expression", "javascript:", "data:", "image(", "image-set(", "src("} {
		if strings.Contains(value, forbidden) {
			return false
		}
	}
	return true
}

func svgDimensions(attrs []xml.Attr) (int, int) {
	var width, height float64
	var viewBox []string
	for _, attr := range attrs {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "width":
			width, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(attr.Value), "px"), 64)
		case "height":
			height, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(attr.Value), "px"), 64)
		case "viewBox":
			viewBox = strings.Fields(strings.ReplaceAll(attr.Value, ",", " "))
		}
	}
	if len(viewBox) == 4 {
		var box [4]float64
		for i, field := range viewBox {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return 0, 0
			}
			box[i] = value
		}
		if box[2] <= 0 || box[3] <= 0 {
			return 0, 0
		}
		if width > 0 && height <= 0 {
			height = width * box[3] / box[2]
		} else if height > 0 && width <= 0 {
			width = height * box[2] / box[3]
		}
		if width <= 0 {
			width = box[2]
		}
		if height <= 0 {
			height = box[3]
		}
	} else if len(viewBox) != 0 {
		return 0, 0
	}
	if math.IsNaN(width) || math.IsNaN(height) || width <= 0 || height <= 0 || width > 1e7 || height > 1e7 {
		return 0, 0
	}
	return int(math.Ceil(width)), int(math.Ceil(height))
}
