package discord

import (
	"fmt"

	"github.com/SevenTV/ServerGo/src/k8s"
	dgo "github.com/SevenTV/discordgo"
	log "github.com/sirupsen/logrus"
)

func RegisterCommands() error {
	commands := []*dgo.ApplicationCommand{
		{
			Name:        "scale",
			Description: "[Sysadmin] Scale a service",
			Options: []*dgo.ApplicationCommandOption{
				{
					Type:        dgo.ApplicationCommandOptionString,
					Name:        "service",
					Description: "Select which service to scale",
					Required:    true,
					Choices: []*dgo.ApplicationCommandOptionChoice{
						{
							Name:  "API",
							Value: "api",
						},
					},
				},
				{
					Type:        dgo.ApplicationCommandOptionInteger,
					Name:        "replicas",
					Description: "The new amount of replicas",
					Required:    true,
				},
			},
		},
		{
			Name:        "restart",
			Description: "[Sysadmin] Restart a service",
			Options: []*dgo.ApplicationCommandOption{
				{
					Type:        dgo.ApplicationCommandOptionString,
					Name:        "service",
					Description: "Select which service to scale",
					Required:    true,
					Choices: []*dgo.ApplicationCommandOptionChoice{
						{
							Name:  "API",
							Value: "api",
						},
					},
				},
			},
		},
	}

	commandHandlers := map[string]func(s *dgo.Session, i *dgo.InteractionCreate){
		"scale": func(s *dgo.Session, i *dgo.InteractionCreate) {
			serviceName := i.Data.Options[0].Value
			replicas := i.Data.Options[1].IntValue()

			if serviceName == "api" {
				err := k8s.Scale(uint32(replicas))
				if err != nil {
					SendInteractionError(s, i.Interaction, err)
					return
				}
			}

			if err := s.InteractionRespond(i.Interaction, &dgo.InteractionResponse{
				Type: dgo.InteractionResponseChannelMessageWithSource,
				Data: &dgo.InteractionApplicationCommandResponseData{
					Content: fmt.Sprintf("**[sysadmin]** scaled **%v** to **%d** replicas", serviceName, replicas),
				},
			}); err != nil {
				log.Errorf("discord, interact reply error, err=%v", err)
			}
		},
	}

	for _, com := range commands {
		_, err := d.ApplicationCommandCreate(d.State.User.ID, systemGuildID, com)
		if err != nil {
			log.Errorf("discord, could not create command %v, err=%v", com.Name, err)
		}
	}

	// Handle command
	d.AddHandler(func(s *dgo.Session, i *dgo.InteractionCreate) {
		fmt.Println("hi", *i, i.Data.Name)
		if h, ok := commandHandlers[i.Data.Name]; ok {
			h(s, i)
		}
	})

	return nil
}

func SendInteractionError(s *dgo.Session, i *dgo.Interaction, err error) {
	s.InteractionRespond(i, &dgo.InteractionResponse{
		Type: dgo.InteractionResponseChannelMessageWithSource,
		Data: &dgo.InteractionApplicationCommandResponseData{
			Content: fmt.Sprintf("**[failure]** %v", err.Error()),
			Flags:   64,
		},
	})
}
