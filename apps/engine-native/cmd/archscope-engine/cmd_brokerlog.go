package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/brokerlog"
)

func init() {
	group := &cobra.Command{Use: "broker-log", Short: "Broker and streaming middleware log analysis commands."}
	var in, out, format string
	var topN int
	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze Kafka, RabbitMQ, Pulsar, NATS, or ActiveMQ evidence.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := brokerlog.Analyze(in, brokerlog.Options{Format: format, TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to broker log or diagnostics file")
	analyze.Flags().StringVar(&format, "format", "auto", "auto|kafka|rabbitmq|rabbitmq-diagnostics-json|pulsar|nats|activemq")
	analyze.Flags().IntVar(&topN, "top-n", brokerlog.DefaultTopN, "maximum rows per table")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
