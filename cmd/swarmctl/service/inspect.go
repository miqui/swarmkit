package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/cmd/swarmctl/common"
	"github.com/docker/swarmkit/cmd/swarmctl/task"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

func printServiceSummary(service *api.Service) {
	w := tabwriter.NewWriter(os.Stdout, 8, 8, 8, ' ', 0)
	defer w.Flush()

	task := service.Spec.Task
	common.FprintfIfNotEmpty(w, "ID\t: %s\n", service.ID)
	common.FprintfIfNotEmpty(w, "Name\t: %s\n", service.Spec.Annotations.Name)
	if len(service.Spec.Annotations.Labels) > 0 {
		fmt.Fprintln(w, "Labels\t")
		for k, v := range service.Spec.Annotations.Labels {
			fmt.Fprintf(w, "  %s\t: %s\n", k, v)
		}
	}
	common.FprintfIfNotEmpty(w, "Replicas\t: %s\n", getServiceReplicasTxt(service))
	fmt.Fprintln(w, "Template\t")
	fmt.Fprintln(w, " Container\t")
	ctr := service.Spec.Task.GetContainer()
	common.FprintfIfNotEmpty(w, "  Image\t: %s\n", ctr.Image)
	common.FprintfIfNotEmpty(w, "  Command\t: %q\n", strings.Join(ctr.Command, " "))
	common.FprintfIfNotEmpty(w, "  Args\t: [%s]\n", strings.Join(ctr.Args, ", "))
	common.FprintfIfNotEmpty(w, "  Env\t: [%s]\n", strings.Join(ctr.Env, ", "))
	if task.Placement != nil {
		common.FprintfIfNotEmpty(w, "  Constraints\t: %s\n", strings.Join(task.Placement.Constraints, ", "))
	}

	if task.Resources != nil {
		res := task.Resources
		fmt.Fprintln(w, "  Resources\t")
		printResources := func(w io.Writer, r *api.Resources) {
			if r.NanoCPUs != 0 {
				fmt.Fprintf(w, "      CPU\t: %g\n", float64(r.NanoCPUs)/1e9)
			}
			if r.MemoryBytes != 0 {
				fmt.Fprintf(w, "      Memory\t: %s\n", humanize.IBytes(uint64(r.MemoryBytes)))
			}
		}
		if res.Reservations != nil {
			fmt.Fprintln(w, "    Reservations:\t")
			printResources(w, res.Reservations)
		}
		if res.Limits != nil {
			fmt.Fprintln(w, "    Limits:\t")
			printResources(w, res.Limits)
		}
	}
	if len(service.Spec.Networks) > 0 {
		fmt.Fprintf(w, "  Networks:\t")
		for _, n := range service.Spec.Networks {
			fmt.Fprintf(w, " %s", n.Target)
		}
	}

	if service.Endpoint != nil && len(service.Endpoint.Ports) > 0 {
		fmt.Fprintln(w, "\nPorts:")
		for _, port := range service.Endpoint.Ports {
			fmt.Fprintf(w, "    - Name\t= %s\n", port.Name)
			fmt.Fprintf(w, "      Protocol\t= %s\n", port.Protocol)
			fmt.Fprintf(w, "      Port\t= %d\n", port.TargetPort)
			fmt.Fprintf(w, "      SwarmPort\t= %d\n", port.PublishedPort)
		}
	}

	if len(ctr.Mounts) > 0 {
		fmt.Fprintln(w, "  Mounts:")
		for _, v := range ctr.Mounts {
			fmt.Fprintf(w, "    - target = %s\n", v.Target)
			fmt.Fprintf(w, "      source = %s\n", v.Source)
			fmt.Fprintf(w, "      writable = %v\n", v.Writable)
			fmt.Fprintf(w, "      type = %v\n", strings.ToLower(v.Type.String()))
		}
	}
}

var (
	inspectCmd = &cobra.Command{
		Use:   "inspect <service ID>",
		Short: "Inspect a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("service ID missing")
			}

			flags := cmd.Flags()

			all, err := flags.GetBool("all")
			if err != nil {
				return err
			}

			c, err := common.Dial(cmd)
			if err != nil {
				return err
			}

			res := common.NewResolver(cmd, c)

			service, err := getService(common.Context(cmd), c, args[0])
			if err != nil {
				return err
			}

			// TODO(aluzzardi): This should be implemented as a ListOptions filter.
			r, err := c.ListTasks(common.Context(cmd), &api.ListTasksRequest{})
			if err != nil {
				return err
			}
			tasks := []*api.Task{}
			for _, t := range r.Tasks {
				if t.ServiceID != service.ID {
					continue
				}
				tasks = append(tasks, t)
			}

			printServiceSummary(service)
			if len(tasks) > 0 {
				fmt.Printf("\n")
				task.Print(tasks, all, res)
			}

			return nil
		},
	}
)

func init() {
	inspectCmd.Flags().BoolP("all", "a", false, "Show all tasks (default shows just running)")
}
