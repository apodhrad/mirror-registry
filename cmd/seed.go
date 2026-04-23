package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Push seed images from a local archive into a running Quay registry.",
	Run: func(cmd *cobra.Command, args []string) {
		seed()
	},
}

func init() {
	rootCmd.AddCommand(seedCmd)

	seedCmd.Flags().StringVarP(&targetHostname, "targetHostname", "H", getFQDN(), "The hostname of the target where Quay is running. This defaults to $HOST")
	seedCmd.Flags().StringVarP(&targetUsername, "targetUsername", "u", os.Getenv("USER"), "The user on the target host which will be used for SSH. This defaults to $USER")
	seedCmd.Flags().StringVarP(&sshKey, "ssh-key", "k", os.Getenv("HOME")+"/.ssh/quay_installer", "The path of your ssh identity key. This defaults to ~/.ssh/quay_installer")

	seedCmd.Flags().StringVarP(&initUser, "initUser", "", "init", "The username to authenticate against Quay.")
	seedCmd.Flags().StringVarP(&initPassword, "initPassword", "", "", "The password to authenticate against Quay.")
	seedCmd.Flags().StringVarP(&quayHostname, "quayHostname", "", "", "The Quay registry hostname (e.g. myhost.example.com:8443). Defaults to <targetHostname>:8443")

	seedCmd.Flags().StringVarP(&seedImageArchivePath, "seed-image-archive", "", "", "Path to the seed images archive. Defaults to seed-images.tar next to the binary.")
	seedCmd.Flags().StringVarP(&quayRoot, "quayRoot", "r", "~/quay-install", "The folder where quay persistent data are saved.")
	seedCmd.Flags().StringVarP(&quayStorage, "quayStorage", "", "quay-storage", "The Quay storage volume name or path.")
	seedCmd.Flags().BoolVarP(&askBecomePass, "askBecomePass", "", false, "Whether or not to ask for sudo password during SSH connection.")
	seedCmd.Flags().StringVarP(&additionalArgs, "additionalArgs", "", "", "Additional arguments to append to the ansible-playbook call.")
}

func seed() {

	if initPassword == "" {
		check(errors.New("--initPassword is required for the seed command"))
	}

	// Ensure the Ansible EE is available (loaded during install, or load from tar)
	err := ensureExecutionEnvironment()
	check(err)

	// Set quayHostname if not already set
	if quayHostname == "" {
		quayHostname = targetHostname + ":8443"
	}
	if !strings.Contains(quayHostname, ":") {
		quayHostname = quayHostname + ":8443"
	}

	// Check that SSH key is present, and generate if not
	err = loadSSHKeys()
	check(err)

	// Handle Seed Image Archive
	var seedImageArchiveMountFlag string
	if seedImageArchivePath == "" {
		execPath, err := os.Executable()
		check(err)
		defaultSeedArchivePath := path.Join(path.Dir(execPath), "seed-images.tar")
		if pathExists(defaultSeedArchivePath) {
			seedImageArchivePath = defaultSeedArchivePath
		}
	} else {
		if !pathExists(seedImageArchivePath) {
			check(errors.New("Could not find seed-images.tar at " + seedImageArchivePath))
		}
	}
	if seedImageArchivePath == "" {
		check(errors.New("No seed image archive found. Provide one via --seed-image-archive or place seed-images.tar next to the binary."))
	}
	seedImageArchiveMountFlag = fmt.Sprintf(" -v %s:/runner/seed-images.tar", seedImageArchivePath)
	log.Info("Found seed image archive at " + seedImageArchivePath)
	setSELinux(seedImageArchivePath)

	// Set askBecomePass flag if true
	var askBecomePassFlag string
	if askBecomePass {
		askBecomePassFlag = "-K"
	}

	log.Printf("Seeding registry images. This may take some time. Run with -v for verbose output.")
	podmanCmd := fmt.Sprintf(`podman run `+
		`--rm --interactive --tty `+
		`--workdir /runner/project `+
		`--net host `+
		seedImageArchiveMountFlag+
		` -v %s:/runner/env/ssh_key `+
		`-e RUNNER_OMIT_EVENTS=False `+
		`-e RUNNER_ONLY_FAILED_EVENTS=False `+
		`-e ANSIBLE_HOST_KEY_CHECKING=False `+
		`-e ANSIBLE_CONFIG=/runner/project/ansible.cfg `+
		fmt.Sprintf("-e ANSIBLE_NOCOLOR=%t ", noColor)+
		`--quiet `+
		`--name ansible_runner_instance `+
		fmt.Sprintf("%s ", eeImage)+
		`ansible-playbook -i %s@%s, --private-key /runner/env/ssh_key -e "init_user=%s init_password=%s quay_hostname=%s local_install=%s quay_root=%s quay_storage=%s" seed_mirror_appliance.yml %s %s`,
		sshKey, targetUsername, targetHostname, initUser, initPassword, quayHostname, strconv.FormatBool(isLocalInstall()), quayRoot, quayStorage, askBecomePassFlag, additionalArgs)

	log.Debug("Running command: " + podmanCmd)
	cmd := exec.Command("bash", "-c", podmanCmd)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	check(err)

	log.Printf("Seed images successfully pushed to %s", "https://"+quayHostname)
}
