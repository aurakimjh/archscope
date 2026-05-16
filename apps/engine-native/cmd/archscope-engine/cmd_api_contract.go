package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/apicontract"
)

func init() {
	group := &cobra.Command{
		Use:   "api-contract",
		Short: "OpenAPI and AsyncAPI contract evidence analysis commands.",
		Long:  "Compare OpenAPI/AsyncAPI specifications with access-log and broker AnalysisResult evidence.",
	}

	var openapiPath, accessResultPath, asyncapiPath, brokerResultPath, out string
	var topN int
	var slowMS, errorRate float64
	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Detect undocumented, unused, slow, and high-error API/event contract evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if openapiPath == "" && asyncapiPath == "" {
				return fmt.Errorf("at least one of --openapi or --asyncapi is required")
			}
			result, err := apicontract.Analyze(apicontract.Options{
				OpenAPIPath:        openapiPath,
				AccessResultPath:   accessResultPath,
				AsyncAPIPath:       asyncapiPath,
				BrokerResultPath:   brokerResultPath,
				TopN:               topN,
				SlowThresholdMS:    slowMS,
				ErrorRateThreshold: errorRate,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&openapiPath, "openapi", "", "OpenAPI JSON/YAML path")
	analyze.Flags().StringVar(&accessResultPath, "access-result", "", "access_log AnalysisResult JSON path")
	analyze.Flags().StringVar(&asyncapiPath, "asyncapi", "", "AsyncAPI JSON/YAML path")
	analyze.Flags().StringVar(&brokerResultPath, "broker-result", "", "broker_log AnalysisResult JSON path")
	analyze.Flags().IntVar(&topN, "top-n", 100, "maximum rows per table")
	analyze.Flags().Float64Var(&slowMS, "slow-ms", 1000, "slow API threshold in milliseconds")
	analyze.Flags().Float64Var(&errorRate, "error-rate-percent", 5, "high-error API threshold in percent")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
