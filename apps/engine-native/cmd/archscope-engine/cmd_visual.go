package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"github.com/spf13/cobra"
)

type visualSpecFile struct {
	Spec visuals.Spec `json:"spec"`
	View visuals.View `json:"view"`
}

func init() {
	group := &cobra.Command{Use: "visual", Short: "Pinned report specifications and saved visual artifacts; new image rendering/clipboard require desktop."}
	var input, output, query, presentation, format, target string
	var page int
	group.PersistentFlags().StringVar(&target, "target-in", "", "second saved result for strict profile diff")
	group.PersistentFlags().StringVar(&input, "in", "", "saved result/spec/artifact file")
	group.PersistentFlags().StringVar(&output, "out", "-", "new output path or stdout")
	group.PersistentFlags().StringVar(&query, "query", `{"kind":"provider_table","provider":"generic:summary","offset":0,"limit":25}`, "bounded explicit visual query")
	group.PersistentFlags().StringVar(&presentation, "presentation", `{}`, "presentation options applied over versioned defaults")
	group.PersistentFlags().StringVar(&format, "format", "json", "saved artifact export: json/spec/view/html/tsv/png/svg/zip; images are never regenerated")
	group.PersistentFlags().IntVar(&page, "page", 0, "zero-based saved image index")
	publish := func(cmd *cobra.Command, body []byte) error {
		if output == "-" {
			_, e := cmd.OutOrStdout().Write(body)
			return e
		}
		root, e := cases.DefaultRoot()
		if e != nil {
			return e
		}
		return localfiles.Publish(cmd.Context(), output, body, root)
	}
	group.AddCommand(&cobra.Command{Use: "create-spec", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		body, e := readMetricDocument(input, 64<<20)
		if e != nil {
			return e
		}
		c, r, e := catalog.Builtin()
		if e != nil {
			return e
		}
		root, e := os.MkdirTemp("", "visual-spec-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(root)
		service, e := analysis.New(root, c, r)
		if e != nil {
			return e
		}
		defer service.Close()
		loaded, e := service.LoadSnapshot(cmd.Context(), body)
		if e != nil {
			return e
		}
		var q visuals.Query
		if e = visuals.Strict([]byte(query), &q, 128<<10); e != nil {
			return e
		}
		p := visuals.DefaultPresentation()
		if e = visuals.Strict([]byte(presentation), &p, 32<<10); e != nil {
			return e
		}
		targetHandle := ""
		if target != "" {
			body, err := readMetricDocument(target, 64<<20)
			if err != nil {
				return err
			}
			other, err := service.LoadSnapshot(cmd.Context(), body)
			if err != nil {
				return err
			}
			targetHandle = other.Handle
		}
		view, e := service.VisualView(cmd.Context(), loaded.Handle, targetHandle, q)
		if e != nil {
			return e
		}
		spec, e := visuals.NewSpec(view, p, "")
		if e != nil {
			return e
		}
		out, e := json.Marshal(visualSpecFile{spec, view})
		if e != nil {
			return e
		}
		return publish(cmd, append(out, '\n'))
	}})
	group.AddCommand(&cobra.Command{Use: "validate", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		body, e := readMetricDocument(input, visuals.MaxViewBytes+128<<10)
		if e != nil {
			return e
		}
		var doc visualSpecFile
		if e = visuals.Strict(body, &doc, visuals.MaxViewBytes+128<<10); e != nil {
			return e
		}
		if e = doc.Spec.Validate(doc.View, true); e != nil {
			return e
		}
		body, e = json.Marshal(map[string]any{"valid": true, "exact_dependencies_available": doc.Spec.Reproducible(), "new_image_rendering": "desktop_only", "view_sha256": doc.Spec.ViewSHA256})
		if e != nil {
			return e
		}
		return publish(cmd, append(body, '\n'))
	}})
	for _, operation := range []string{"inspect", "export"} {
		group.AddCommand(&cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			raw, a, e := visuals.ReadLocalArtifact(input)
			if e != nil {
				return e
			}
			if operation == "inspect" {
				body, e := json.Marshal(a.Receipt)
				if e != nil {
					return e
				}
				return publish(cmd, append(body, '\n'))
			}
			var body []byte
			switch format {
			case "json":
				body, e = json.MarshalIndent(a.Receipt, "", "  ")
			case "spec":
				body = a.Files["spec.json"]
			case "view":
				body = a.Files["view.json"]
			case "html":
				body = []byte(visuals.StandaloneHTML(a.View, a.Receipt.Spec))
			case "tsv":
				body = visuals.TSV(a.View)
			case "zip":
				body = raw
			case "png", "svg":
				if page < 0 || page >= a.Receipt.Pages {
					return fmt.Errorf("VISUAL_PAGE_INVALID")
				}
				body = a.Files[fmt.Sprintf("image-%02d.%s", page+1, format)]
			default:
				return fmt.Errorf("VISUAL_EXPORT_UNSUPPORTED")
			}
			if e != nil {
				return e
			}
			if output != "-" && filepath.Ext(output) == "" {
				return fmt.Errorf("output extension required")
			}
			return publish(cmd, body)
		}})
	}
	group.AddCommand(&cobra.Command{Use: "render", RunE: func(*cobra.Command, []string) error {
		return fmt.Errorf("VISUAL_DESKTOP_RENDERER_REQUIRED: use the desktop to generate PNG/SVG; CLI can validate/store specs and export already-rendered artifacts")
	}}, &cobra.Command{Use: "clipboard", RunE: func(*cobra.Command, []string) error {
		return fmt.Errorf("VISUAL_CLIPBOARD_UNSUPPORTED: clipboard output requires a native desktop session")
	}})
	rootCmd.AddCommand(group)
}
