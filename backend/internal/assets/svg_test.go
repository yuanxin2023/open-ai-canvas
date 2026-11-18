package assets

import "testing"

func TestInspectStaticSVG(t *testing.T) {
	for _, body := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 120"><defs><linearGradient id="paint"><stop offset="0" stop-color="red"/></linearGradient></defs><path d="M0 0h240v120z" fill="url(#paint)"/></svg>`,
		`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="240px" height="120px"><style>.a{fill:url('#paint')}</style><g><text class="a">猫</text></g></svg>`,
	} {
		w, h, err := InspectStaticSVG([]byte(body))
		if err != nil || w != 240 || h != 120 {
			t.Fatalf("dimensions=(%d,%d), err=%v", w, h, err)
		}
	}
}

func TestInspectStaticSVGRejectsActiveOrNonVectorContent(t *testing.T) {
	for _, content := range []string{
		`<script>alert(1)</script>`, `<g onload="alert(1)"/>`,
		`<foreignObject><div xmlns="http://www.w3.org/1999/xhtml"/></foreignObject>`,
		`<image href="data:image/png;base64,AAAA"/>`, `<use href="https://example.com/x.svg#a"/>`,
		`<style>@import 'https://example.com/x.css';</style>`,
		`<style>.a{fill:URL(https://example.com/a)}</style>`,
		`<style>.a{background-image:image-set("remote.png" 1x)}</style>`,
		`<style>.a{fill:u\72l(https://example.com/a)}</style>`,
		`<style>.a{fill:u<!-- split -->rl(https://example.com/a)}</style>`,
		`<animate attributeName="href"/>`, `<g xml:base="https://example.com/"/>`,
		`<g xmlns="http://example.com/"/>`, `<path fill="url(data:image/png;base64,AAAA)"/>`,
	} {
		t.Run(content, func(t *testing.T) {
			if _, _, err := InspectStaticSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` + content + `</svg>`)); err == nil {
				t.Fatal("unsafe content accepted")
			}
		})
	}
	for _, document := range []string{
		"GIF89a", "\x89PNG", `<html/>`, `<svg/>`,
		`<!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="NaN" height="10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 -1 10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="bad 0 10 10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/><svg/>`,
	} {
		if _, _, err := InspectStaticSVG([]byte(document)); err == nil {
			t.Fatalf("invalid document accepted: %s", document)
		}
	}
}
