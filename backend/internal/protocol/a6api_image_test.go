package protocol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestA6APIImagePackagedParametersAndTransparentRequests(t *testing.T) {
	adapter := officialPackageAdapter(t, "a6api.yingce-plugin", "a6api-image")
	fields := map[string]ImageParameter{}
	for _, field := range adapter.Metadata().ImageParameters {
		fields[field.Name] = field
	}
	for _, name := range []string{"size", "imageResolution", "quality", "count", "transparentBackground", "outputFormat"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("packaged plugin is missing %s; existing image controls must survive the dynamic contract", name)
		}
	}
	if fields["transparentBackground"].IsEnabled() || fields["transparentBackground"].Default != false {
		t.Fatal("transparent output must require model-level opt-in and default to off")
	}

	for _, editing := range []bool{false, true} {
		for _, state := range []string{"unset", "off", "on"} {
			name := "generation/" + state
			if editing {
				name = "edit/" + state
			}
			t.Run(name, func(t *testing.T) {
				request := GenerationRequest{
					Capability: CapabilityImage,
					Model:      "gpt-image-2",
					Prompt:     "keep the subject with a transparent background",
					ImageParameters: map[string]any{
						"size": "1024x1024", "quality": "high", "count": 2, "outputFormat": "png",
					},
				}
				if state != "unset" {
					request.ImageParameters["transparentBackground"] = state == "on"
				}
				if editing {
					request.Images = []MediaReference{
						{DataURL: "data:image/png;base64,aW1hZ2U=", Role: "edit_source", Order: 0},
						{DataURL: "data:image/png;base64,bWFzaw==", Role: "mask", Order: 1},
					}
				}
				spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
				if err != nil {
					t.Fatal(err)
				}
				body := manifestTestBody(t, spec)
				if body["size"] != "1024x1024" || body["quality"] != "high" || body["n"] != float64(2) || body["output_format"] != "png" {
					t.Fatalf("existing image parameters or PNG output were lost: %#v", body)
				}
				if state == "on" {
					if body["background"] != "transparent" {
						t.Fatalf("transparent request did not reach the upstream body: %#v", body)
					}
				} else if _, exists := body["background"]; exists {
					t.Fatalf("off/unset must omit background: %#v", body)
				}
				if editing {
					if spec.Path != "/images/edits" || spec.ContentType != "multipart/form-data" || len(spec.Files) != 2 || spec.Files[0].Name != "image" || spec.Files[1].Name != "mask" {
						t.Fatalf("edit source or mask was lost: %#v", spec)
					}
				} else if spec.Path != "/images/generations" || spec.ContentType != "application/json" || len(spec.Files) != 0 {
					t.Fatalf("unexpected generation transport: %#v", spec)
				}
			})
		}
	}
}

func TestA6APIImageRejectsInvalidParametersAndPinsPNG(t *testing.T) {
	adapter := officialPackageAdapter(t, "a6api.yingce-plugin", "a6api-image")
	for name, values := range map[string]map[string]any{
		"lossy output":       {"outputFormat": "jpeg"},
		"invalid boolean":    {"transparentBackground": "true"},
		"out of range count": {"count": 101},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{
				Capability: CapabilityImage, Model: "gpt-image-2", Prompt: "subject", ImageParameters: values,
			}})
			if err == nil {
				t.Fatal("invalid image parameter was accepted")
			}
		})
	}
	// Old extension metadata must not override the PNG response contract.
	spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{
		Capability: CapabilityImage, Model: "gpt-image-2", Prompt: "subject",
		ImageParameters: map[string]any{"transparentBackground": true},
		ProviderOptions: map[string]map[string]any{"a6api-image": {"output_format": "jpeg"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if body := manifestTestBody(t, spec); body["output_format"] != "png" || body["background"] != "transparent" {
		t.Fatalf("stale options overrode transparent PNG output: %#v", body)
	}
}

func TestA6APIImageResponsePreservesPNGAlphaBytes(t *testing.T) {
	adapter := officialPackageAdapter(t, "a6api.yingce-plugin", "a6api-image")
	input := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	input.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 0})
	input.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, input); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(encoded.Bytes())}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.ParseCreate(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result == nil || len(result.Result.Images) != 1 {
		t.Fatalf("missing PNG result: %#v", result)
	}
	dataURL := result.Result.Images[0].DataURL
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("wrong PNG MIME type: %q", dataURL)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, "data:image/png;base64,"))
	if err != nil || !bytes.Equal(decoded, encoded.Bytes()) {
		t.Fatalf("PNG bytes were changed: %v", err)
	}
	output, err := png.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatal(err)
	}
	for x, want := range []uint8{0, 128} {
		if got := color.NRGBAModel.Convert(output.At(x, 0)).(color.NRGBA).A; got != want {
			t.Fatalf("pixel %d alpha = %d, want %d", x, got, want)
		}
	}
}

func TestA6APIChatStillBuildsTextRequests(t *testing.T) {
	adapter := officialPackageAdapter(t, "a6api.yingce-plugin", "a6api-chat")
	spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{
		Capability: CapabilityText, Model: "text-model", Messages: []Message{{Role: "user", Content: "hello"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Path != "/chat/completions" || len(manifestTestBody(t, spec)["messages"].([]any)) != 1 {
		t.Fatalf("text provider changed: %#v", spec)
	}
}
