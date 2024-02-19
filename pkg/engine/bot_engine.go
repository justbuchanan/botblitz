package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	common "github.com/mitchwebster/botblitz/pkg/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const pyServerHostAndPort = "localhost:8080"

const botResourceFolderName = "/tmp"
const botFileRelativePath = botResourceFolderName + "/bot.py" // source code name passed in resource folder
const containerServerPort = "8080"
const botResourceFolderNameInContainer = "/botblitz"

type BotEngineSettings struct {
	VerboseLoggingEnabled bool
}

type BotEngine struct {
	botResults      map[string]map[string][]*common.FantasySelections // Bot -> Simulation -> Simulation Selections per iteration
	settings        BotEngineSettings
	bots            []*common.Bot
	simulations     []*common.Simulation
	sourceCodeCache map[string][]byte
}

func NewBotEngine(simulations []*common.Simulation, bots []*common.Bot, settings BotEngineSettings) *BotEngine {
	return &BotEngine{
		settings:        settings,
		bots:            bots,
		simulations:     simulations,
		botResults:      make(map[string]map[string][]*common.FantasySelections),
		sourceCodeCache: make(map[string][]byte),
	}
}

func (e BotEngine) Summarize() string {
	var builder strings.Builder

	// write configs
	fmt.Fprintf(&builder, "Engine Summary\n\n")

	// Print settings
	fmt.Fprintf(&builder, "Settings:\n")
	fmt.Fprintf(&builder, "\tNumSimulations: %d\n", len(e.simulations))
	fmt.Fprintf(&builder, "\tVerboseLoggingEnabled: %t\n", e.settings.VerboseLoggingEnabled)

	fmt.Fprintf(&builder, "\nBots:\n")
	fmt.Fprintf(&builder, "\tCount: %d\n", len(e.bots))
	for _, obj := range e.bots {
		fmt.Fprintf(&builder, "\t - %s\n", obj.Id)
	}

	return builder.String()
}

func (e BotEngine) Run() error {
	err := performValidations(e)
	if err != nil {
		return err
	}

	return run(e)
}

func (e BotEngine) PrintResults() {
	for botId, simulationMap := range e.botResults {
		fmt.Printf("%s:\n", botId)
		for simulationId, results := range simulationMap {
			fmt.Printf("\t%s: %s\n", simulationId, results)
		}
	}
}

func performValidations(e BotEngine) error {
	botValidation := common.ValidateBotConfigs(e.bots)
	if !botValidation {
		return errors.New("Bot validation failed, please check provided bots")
	}

	simulationValidation := common.ValidateSimulation(e.simulations)
	if !simulationValidation {
		return errors.New("Simulation validation failed, please check provided simulations")
	}

	return nil
}

func run(e BotEngine) error {
	if e.settings.VerboseLoggingEnabled {
		fmt.Println("Running engine")
	}

	err := collectBotResources(e)
	if err != nil {
		return err
	}

	err = initializeBots(e)
	if err != nil {
		return err
	}

	for _, bot := range e.bots {
		containerId, err := startBotContainer(bot, e)
		if err != nil {
			return err
		}

		fmt.Printf("Setup bot: %s\n", bot.Id)
		fmt.Printf("Bot details: Username: %s, Repo: %s, Fantasy Team Id: %d\n", bot.SourceRepoUsername, bot.SourceRepoName, bot.FantasyTeamId)
		fmt.Printf("Using a %s source to find %s\n", bot.SourceType, bot.SourcePath)

		err = runSimulationsOnBot(bot, e)
		if err != nil {
			fmt.Println("Failed to run simulations on bot")
			fmt.Println(err)
		}

		err = shutDownAndCleanBotServer(bot, containerId)
		if err != nil {
			fmt.Println("CRITICAL!! Failed to clean after bot run")
			return err
		}
	}

	return nil
}

func collectBotResources(e BotEngine) error {
	folderPath, err := buildLocalAbsolutePath(botResourceFolderName)
	if err != nil {
		return err
	}

	// Clear temp folder
	err = os.RemoveAll(folderPath)
	if err != nil {
		// Non-existence errors are ok
		if !os.IsNotExist(err) {
			return err
		}
	}

	err = os.Mkdir(folderPath, os.ModePerm)
	if err != nil {
		return err
	}

	// TODO: put any resources we want to expose to the bot in this directory

	return nil
}

func initializeBots(e BotEngine) error {
	fmt.Printf("\n-----------------------------------------\n")
	fmt.Println("Initializing Bots")

	for _, bot := range e.bots {
		e.botResults[bot.Id] = make(map[string][]*common.FantasySelections)
		byteCode, err := fetchSourceCode(bot, e)
		if err != nil {
			fmt.Printf("Failed to retrieve bot source code for (%s)\n", bot.Id)
			return err
		}

		e.sourceCodeCache[bot.Id] = byteCode
	}

	fmt.Printf("\n-----------------------------------------\n")

	return nil
}

func shutDownAndCleanBotServer(bot *common.Bot, containerId string) error {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer apiClient.Close()

	apiClient.NegotiateAPIVersion(context.Background())

	fmt.Println("Killing container")
	err = apiClient.ContainerKill(context.Background(), containerId, "")
	if err != nil {
		return err
	}

	fmt.Println("Force deleting container")
	err = apiClient.ContainerRemove(context.Background(), containerId, container.RemoveOptions{Force: true})
	if err != nil {
		return err
	}

	fmt.Println("Force deleted container")

	err = cleanBotResources()
	if err != nil {
		return err
	}

	fmt.Printf("Finished cleaning server for bot (%s)\n", bot.Id)
	fmt.Printf("\n-----------------------------------------\n")

	return nil
}

func runSimulationsOnBot(bot *common.Bot, e BotEngine) error {

	for _, simulation := range e.simulations {
		fmt.Printf("\n-----------------------------------------\n")
		fmt.Printf("Running simulation (%s) on bot (%s)\n", simulation.Id, bot.Id)
		selectionsForSimulation := []*common.FantasySelections{}

		for iteration := uint32(1); iteration <= simulation.NumIterations; iteration++ {
			fmt.Printf("\n\tIteration (%d): Making gRPC call\n", iteration)
			selectionsForIteration, err := callBotRPC(simulation.Landscape)
			if err != nil {
				fmt.Printf("\tIteration (%d): Failed to make gRPC call\n", iteration)
				fmt.Println(err)
				selectionsForSimulation = append(selectionsForSimulation, nil)
			} else {
				fmt.Printf("\tIteration (%d): bot ran successfully!\n", iteration)
				selectionsForSimulation = append(selectionsForSimulation, selectionsForIteration)
			}

		}

		e.botResults[bot.Id][simulation.Id] = selectionsForSimulation
		fmt.Printf("\n-----------------------------------------\n") // Add formatting to make separate runs clear
	}

	return nil
}

func fetchSourceCode(bot *common.Bot, e BotEngine) ([]byte, error) {
	var botCode []byte

	if bot.SourceType == common.Bot_REMOTE {
		downloadedSourceCode, err := DownloadGithubSourceCode(bot.SourceRepoUsername, bot.SourceRepoName, bot.SourcePath, e.settings.VerboseLoggingEnabled)
		if err != nil {
			return nil, err
		}

		botCode = downloadedSourceCode
	} else {
		absPath, err := buildLocalAbsolutePath(bot.SourcePath)
		if err != nil {
			return nil, err
		}

		bytes, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}

		botCode = bytes
	}

	fmt.Printf("Successfully retrieved source code for bot (%s)\n", bot.Id)
	return botCode, nil
}

func startBotContainer(bot *common.Bot, e BotEngine) (string, error) {
	fmt.Printf("\n-----------------------------------------\n")
	fmt.Printf("Bootstrapping server for bot (%s)\n", bot.Id)

	botCode := e.sourceCodeCache[bot.Id]

	fmt.Println("Creating source code file")
	absPath, err := buildLocalAbsolutePath(botFileRelativePath)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(absPath, botCode, 0755)
	if err != nil {
		return "", err
	}

	containerId, err := createAndStartContainer()
	if err != nil {
		return "", err
	}

	return containerId, nil
}

func buildLocalAbsolutePath(relativePath string) (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}

	var trimmedPath = strings.Trim(relativePath, "/")
	return fmt.Sprintf("%s/%s", directory, trimmedPath), nil
}

func cleanBotResources() error {
	absPath, err := buildLocalAbsolutePath(botFileRelativePath)
	if err != nil {
		return err
	}

	err = os.Remove(absPath)
	if err != nil {
		// Non-existence errors are ok
		if !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func callBotRPC(landscape *common.FantasyLandscape) (*common.FantasySelections, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial(pyServerHostAndPort, opts...)
	if err != nil {
		return nil, err
	}

	defer conn.Close()
	client := common.NewAgentServiceClient(conn)

	selections, err := client.PerformFantasyActions(context.Background(), landscape)
	if err != nil {
		return nil, err
	}

	return selections, nil
}

func createAndStartContainer() (string, error) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	apiClient.NegotiateAPIVersion(context.Background())

	hostBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: "8080",
	}

	containerPort, err := nat.NewPort("tcp", containerServerPort)
	if err != nil {
		panic("Unable to get the port")
	}

	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}

	hostMountPath, err := buildLocalAbsolutePath(botResourceFolderName)
	if err != nil {
		panic(err)
	}

	createResponse, _ := apiClient.ContainerCreate(
		context.Background(),
		&container.Config{
			Image: "py-grpc-server",
		},
		&container.HostConfig{
			PortBindings: portBinding,
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeBind,
					ReadOnly: true,
					Source:   hostMountPath,
					Target:   botResourceFolderNameInContainer,
				},
			},
		},
		nil,
		nil,
		"",
	)

	err = apiClient.ContainerStart(context.Background(), createResponse.ID, container.StartOptions{})
	if err != nil {
		fmt.Println(err)
		// TODO: delete the container we created if we can't start it?
	}

	time.Sleep(2 * time.Second) // Give container 2 seconds to start up
	// TODO: could we potentially check that the container is running?

	return createResponse.ID, nil
}
