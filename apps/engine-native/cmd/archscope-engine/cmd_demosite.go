// `demo-site` group — stub. The Python CLI (engines/python/.../cli.py)
// owns the demo-runner today; T-380 will port that to engine-native.
// We keep the group registered so it appears in `--help` and so the
// future port is a one-file change.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const demoSiteStubMessage = "demo runner not yet ported (T-380); use `python -m archscope_engine.cli demo-site` for now"

func init() {
	group := &cobra.Command{
		Use:   "demo-site",
		Short: "Demo-site manifest runner commands (not yet ported).",
		Long: `Demo-site manifest runner — placeholder. The runner is tracked by
T-380; until then, use Python: ` + "`python -m archscope_engine.cli demo-site`" + `.`,
	}

	stub := func(use, short string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s", demoSiteStubMessage)
			},
		}
	}

	group.AddCommand(stub("run", "Run demo-site manifests (not yet ported)."))
	group.AddCommand(stub("mapping", "Print analyzer-type → CLI mapping (not yet ported)."))

	rootCmd.AddCommand(group)
}
