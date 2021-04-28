package telegram

import (
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	cfgs "github.com/skoret/wireguard-bot/internal/wireguard/configs"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log"
	"net"
	"os"
	"os/exec"
)

type handler func(data interface{}) interface{}

type command struct {
	tgbotapi.BotCommand
	text     string
	keyboard *tgbotapi.InlineKeyboardMarkup
	handler  handler
}

func (cmd command) button() tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(cmd.Description, cmd.Command)
}

var (
	MenuCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "menu",
			Description: "bot menu",
		},
		text: "so, what do you want?",
	}
	NewConfigCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "newconfing",
			Description: "create new config file for public server",
		},
		text: "do you want new config for new generated key pair or for your public key?",
	}
	ConfigForNewKeysCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "generatekeys",
			Description: "create new config file for new generated key pair",
		},
		text: "this is your new config for public wireguard vpn server, keep it in secret!",
		handler: func(data interface{}) interface{} {
			// conf file creation
			pri, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				log.Fatalf("failed to generate private key: %v", err)
			}
			address := "10.8.0.3/32"
			clientConfig := cfgs.ClientConfig{
				Address:    address,
				PrivateKey: pri.String(),
				DNS:        []string{"8.8.8.8", "8.8.4.4"},

				PublicKey:  os.Getenv("SERVER_PUB_KEY"),
				AllowedIPs: []string{"0.0.0.0/0"},
			}
			cfgFile, err := cfgs.ProcessClientConfig(clientConfig)
			if err != nil {
				panic(err)
			}

			// wg server conf update
			pub := pri.PublicKey()
			_, ipNet, err := net.ParseCIDR(address)
			if err != nil {
				log.Fatalf("failed to parse ip with mask: %v", err)
			}

			cfg := wgtypes.Config{
				ReplacePeers: false,
				Peers: []wgtypes.PeerConfig{
					{
						PublicKey:                   pub,
						Remove:                      false,
						UpdateOnly:                  false,
						PresharedKey:                nil,
						Endpoint:                    nil,
						PersistentKeepaliveInterval: nil,
						ReplaceAllowedIPs:           false,
						AllowedIPs:                  []net.IPNet{*ipNet},
					},
				},
			}
			c, err := wgctrl.New()
			if err != nil {
				log.Fatalf("failed to open wgctrl: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					panic(err)
				}
			}()
			if err := c.ConfigureDevice("wg0", cfg); err != nil {
				if os.IsNotExist(err) {
					fmt.Println(err)
				} else {
					log.Fatalf("Unknown config error: %v", err)
				}
			}
			fmt.Println("--- WgQuickSave ---")
			cmd := exec.Command("wg-quick", "save", "wg0")
			cmd.Stdout = os.Stdout
			if err := cmd.Run(); err != nil {
				panic(err)
			}
			fmt.Println("-------------------")

			return cfgFile
		},
	}
	ConfigForPublicKeyCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "configforkey",
			Description: "create new config file for given public key",
		},
		text: "send me your wireguard public key, please",
	}
	DonateCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "donate",
			Description: "buy me a beer and get a private wg vpn [WIP]",
		},
		text: "sorry, donations aren't supported yet",
	}
	HelpCmd = command{
		BotCommand: tgbotapi.BotCommand{
			Command:     "help",
			Description: "show bot's functionality with description",
		},
		text: "hi, i'm wireguard bot\n\n" +
			"i can create new wg vpn configuration files for you\n" +
			// TODO: write proper help message
			"TODO: write proper help message",
	}
)

var commands = map[string]*command{
	MenuCmd.Command:               &MenuCmd,
	NewConfigCmd.Command:          &NewConfigCmd,
	ConfigForNewKeysCmd.Command:   &ConfigForNewKeysCmd,
	ConfigForPublicKeyCmd.Command: &ConfigForPublicKeyCmd,
	ConfigForPublicKeyCmd.Command: &ConfigForPublicKeyCmd,
	DonateCmd.Command:             &DonateCmd,
	HelpCmd.Command:               &HelpCmd,
}

// setMyCommands is adapted method from unreleased v5.0.1
// https://github.com/go-telegram-bot-api/telegram-bot-api/commit/4a2c8c4547a868841c1ec088302b23b59443de2b
func setMyCommands(api *tgbotapi.BotAPI) error {
	params := make(tgbotapi.Params)
	data, err := json.Marshal([]command{MenuCmd, NewConfigCmd, DonateCmd, HelpCmd})
	if err != nil {
		return err
	}
	params.AddNonEmpty("commands", string(data))
	_, err = api.MakeRequest("setMyCommands", params)
	if err != nil {
		return err
	}
	return nil
}
