package main

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/sippy/pkg/api"
	bqcachedclient "github.com/openshift/sippy/pkg/bigquery"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/sippy/pkg/flags"
	"github.com/openshift/sippy/pkg/flags/configflags"
	"github.com/openshift/sippy/pkg/variantregistry"
)

type VariantSnapshotFlags struct {
	Path                    string
	BigQueryFlags           *flags.BigQueryFlags
	GoogleCloudFlags        *flags.GoogleCloudFlags
	ConfigFlags             *configflags.ConfigFlags
	ComponentReadinessFlags *flags.ComponentReadinessFlags
}

func NewVariantSnapshotFlags() *VariantSnapshotFlags {
	return &VariantSnapshotFlags{
		BigQueryFlags:           flags.NewBigQueryFlags(),
		GoogleCloudFlags:        flags.NewGoogleCloudFlags(),
		ConfigFlags:             configflags.NewConfigFlags(),
		ComponentReadinessFlags: flags.NewComponentReadinessFlags(),
		Path:                    "pkg/variantregistry/snapshot.yaml",
	}
}

func (f *VariantSnapshotFlags) BindFlags(fs *pflag.FlagSet) {
	f.BigQueryFlags.BindFlags(fs)
	f.GoogleCloudFlags.BindFlags(fs)
	f.ConfigFlags.BindFlags(fs)
	f.ComponentReadinessFlags.BindFlags(fs)
	fs.StringVar(&f.Path, "out", f.Path, "Path to write results to")
}

func NewVariantSnapshotCommand() *cobra.Command {
	f := NewVariantSnapshotFlags()

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Update the variants snapshot with local data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.ConfigFlags.Path == "" {
				return fmt.Errorf("--config is required")
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
			defer cancel()

			cfg, err := f.ConfigFlags.GetConfig()
			if err != nil {
				return err
			}

			opCtx, ctx := bqcachedclient.OpCtxForCronEnv(ctx, "variants snapshot")
			bqClient, err := f.BigQueryFlags.GetBigQueryClient(ctx, opCtx, nil, f.GoogleCloudFlags.ServiceAccountCredentialFile)
			if err != nil {
				return fmt.Errorf("error getting BigQuery client: %w", err)
			}
			releaseConfigs, err := api.GetReleasesFromBigQuery(ctx, bqClient)
			if err != nil {
				return fmt.Errorf("error loading releases from BigQuery: %w", err)
			}
			syntheticReleaseJobOverrides, err := variantregistry.BuildSyntheticReleaseJobOverrides(cfg.Releases, releaseConfigs)
			if err != nil {
				return fmt.Errorf("error building synthetic release job overrides: %w", err)
			}

			views, err := f.ComponentReadinessFlags.ParseViewsFile()
			if err != nil {
				return err
			}

			lgr := log.New()
			snapshot := variantregistry.NewVariantSnapshot(cfg, views.ComponentReadiness, syntheticReleaseJobOverrides, lgr)
			if err := snapshot.Save(f.Path); err != nil {
				lgr.WithError(err).Fatal("error updating snapshot")
			}

			lgr.Infof("variants successfully snapshotted")
			return nil
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}
