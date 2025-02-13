package main

import (
	"context"
	"flag"

	"github.com/connector-recruitment/internal/app/config"
	awsSM "github.com/connector-recruitment/internal/infrastructure/aws/secretsmanager"
	pgRepo "github.com/connector-recruitment/internal/infrastructure/postgres"
	slackInfra "github.com/connector-recruitment/internal/infrastructure/slack"
	"github.com/connector-recruitment/pkg/logger"
	"github.com/connector-recruitment/pkg/slackutil"
	_ "github.com/lib/pq"
)

func main() {
	connectorID := flag.String("connector-id", "", "Connector ID")
	message := flag.String("message", "", "Message to send")
	flag.Parse()

	if *connectorID == "" || *message == "" {
		logger.Fatal().Msg("Both --connector-id and --message must be provided")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	ctx := context.Background()

	db, err := pgRepo.InitDatabase(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer db.Close()

	awsCfg, err := config.LoadAWSConfig(cfg.AWSRegion, cfg.AWSEndpoint)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load AWS config")
	}
	smClient := awsSM.NewClient(awsCfg)

	slackClient := slackInfra.NewSlackClient(
		cfg.SlackBaseURL,
		cfg.SlackClientID,
		cfg.SlackClientSecret,
		cfg.SlackRedirectURL,
		cfg.SlackScopes,
	)

	err = slackutil.SendSlackMessage(ctx, *connectorID, *message, db, smClient, slackClient)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to send Slack message")
	}

	logger.Info().Msg("Slack message sent successfully")
}
