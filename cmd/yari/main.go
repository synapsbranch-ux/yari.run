// Command yari is the yari.run command line client. It uses the same Connect API as every other
// client; it never touches a provider or the database.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
)

type env struct {
	out io.Writer
	api string
	hdr func(h http.Header)
}

func (e *env) opts() connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			e.hdr(req.Header())
			return next(ctx, req)
		}
	}), streamAuth{e})
}

type streamAuth struct{ e *env }

func (s streamAuth) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc { return next }
func (s streamAuth) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		s.e.hdr(conn.RequestHeader())
		return conn
	}
}
func (s streamAuth) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func newEnv(out io.Writer) (*env, error) {
	e := &env{out: out, api: os.Getenv("YARI_API_URL")}
	if e.api == "" {
		e.api = "http://localhost:8080"
	}
	token, dev := os.Getenv("YARI_TOKEN"), os.Getenv("YARI_DEV_SUBJECT")
	switch {
	case token != "":
		e.hdr = func(h http.Header) { h.Set("Authorization", "Bearer "+token) }
	case dev != "":
		e.hdr = func(h http.Header) { h.Set("X-Yari-Dev-Subject", dev) }
	default:
		return nil, errors.New("set YARI_TOKEN (an API token) or, against a dev server, YARI_DEV_SUBJECT")
	}
	return e, nil
}

func newRoot(out io.Writer) *cobra.Command {
	var e *env
	root := &cobra.Command{
		Use:           "yari",
		Short:         "yari.run command line client",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			if e, err = newEnv(out); err != nil {
				return err
			}
			cmd.SetContext(cmd.Context())
			return nil
		},
	}
	root.SetOut(out)

	root.AddCommand(&cobra.Command{
		Use: "whoami", Short: "Show who you are and which organizations you belong to", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := yariv1connect.NewIdentityServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.WhoAmI(cmd.Context(), connect.NewRequest(&yariv1.WhoAmIRequest{}))
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "name:  %s\nemail: %s\n", r.Msg.DisplayName, r.Msg.Email)
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ORGANIZATION\tPRINCIPAL\tKIND")
			for _, m := range r.Msg.Memberships {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", m.OrganizationSlug, m.PrincipalId, strings.ToLower(strings.TrimPrefix(m.PrincipalKind.String(), "PRINCIPAL_KIND_")))
			}
			return tw.Flush()
		},
	})

	orgs := &cobra.Command{Use: "orgs", Short: "Manage organizations"}
	orgs.AddCommand(&cobra.Command{
		Use: "list", Short: "List your organizations", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := yariv1connect.NewHierarchyServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.ListOrganizations(cmd.Context(), connect.NewRequest(&yariv1.ListOrganizationsRequest{}))
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "SLUG\tNAME\tID")
			for _, o := range r.Msg.Organizations {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", o.Slug, o.Name, o.Id)
			}
			return tw.Flush()
		},
	})
	var orgName string
	create := &cobra.Command{
		Use: "create <slug>", Short: "Create an organization. You become its owner", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := orgName
			if name == "" {
				name = args[0]
			}
			c := yariv1connect.NewHierarchyServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.CreateOrganization(cmd.Context(), connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: args[0], Name: name}))
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "created organization %s (%s)\n", r.Msg.Organization.Slug, r.Msg.Organization.Id)
			return nil
		},
	}
	create.Flags().StringVar(&orgName, "name", "", "display name (defaults to the slug)")
	orgs.AddCommand(create)
	root.AddCommand(orgs)

	providers := &cobra.Command{Use: "providers", Short: "Browse the Provider Catalog"}
	var category, kind string
	pl := &cobra.Command{
		Use: "list", Short: "List providers", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := yariv1connect.NewCatalogServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.ListProviders(cmd.Context(), connect.NewRequest(&yariv1.ListProvidersRequest{Category: category, Kind: kind}))
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tCATEGORY\tSTATUS\tKINDS")
			for _, p := range r.Msg.Providers {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Id, p.Category, p.Status, strings.Join(p.Kinds, ","))
			}
			return tw.Flush()
		},
	}
	pl.Flags().StringVar(&category, "category", "", "only this category (paas, cloud, database, storage, ...)")
	pl.Flags().StringVar(&kind, "kind", "", "only providers that support this kind")
	providers.AddCommand(pl)
	root.AddCommand(providers)

	plan := &cobra.Command{Use: "plan", Short: "Work with plans"}
	plan.AddCommand(&cobra.Command{
		Use: "apply <id>", Short: "Apply a plan, or request approval where the environment needs it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := yariv1connect.NewPlanServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.ApplyPlan(cmd.Context(), connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: args[0]}))
			if err != nil {
				return err
			}
			status := strings.ToLower(strings.TrimPrefix(r.Msg.Plan.Status.String(), "PLAN_STATUS_"))
			if r.Msg.RequiresApproval {
				fmt.Fprintf(out, "plan %s is %s: it needs approval from another principal with plan.approve\n", args[0], status)
			} else {
				fmt.Fprintf(out, "plan %s is %s\n", args[0], status)
			}
			return nil
		},
	})
	root.AddCommand(plan)

	ag := &cobra.Command{Use: "agent", Short: "Run the DevOps agent"}
	var envID, model string
	start := &cobra.Command{
		Use: "start --env <environment-id> \"<goal>\"", Short: "Start an agent session", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if envID == "" {
				return errors.New("--env is required")
			}
			c := yariv1connect.NewAgentServiceClient(http.DefaultClient, e.api, e.opts())
			r, err := c.StartSession(cmd.Context(), connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: envID, Goal: args[0], Model: model}))
			if err != nil {
				return err
			}
			s := r.Msg.Session
			fmt.Fprintf(out, "session %s started (runtime %s, model %s)\nwatch it: yari agent watch %s\n", s.Id, s.Runtime, s.Model, s.Id)
			return nil
		},
	}
	start.Flags().StringVar(&envID, "env", "", "environment id")
	start.Flags().StringVar(&model, "model", "", "model alias (see the client's model picker)")
	watch := &cobra.Command{
		Use: "watch <session-id>", Short: "Stream the steps of a session until it ends", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := yariv1connect.NewAgentServiceClient(http.DefaultClient, e.api, e.opts())
			stream, err := c.WatchSession(cmd.Context(), connect.NewRequest(&yariv1.WatchSessionRequest{SessionId: args[0]}))
			if err != nil {
				return err
			}
			defer stream.Close()
			var last yariv1.SessionStatus
			for stream.Receive() {
				m := stream.Msg()
				if st := m.Step; st != nil {
					fmt.Fprintf(out, "%3d  %-11s %-8s %-18s %s\n", st.Seq, enumName(st.Kind.String(), "STEP_KIND_"), enumName(st.Location.String(), "STEP_LOCATION_"), st.Tool, st.Summary)
				}
				last = m.Status
			}
			if err := stream.Err(); err != nil {
				return err
			}
			fmt.Fprintf(out, "session ended: %s\n", enumName(last.String(), "SESSION_STATUS_"))
			return nil
		},
	}
	ag.AddCommand(start, watch)
	root.AddCommand(ag)
	return root
}

func enumName(s, prefix string) string { return strings.ToLower(strings.TrimPrefix(s, prefix)) }

func main() {
	if err := newRoot(os.Stdout).ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", connectMessage(err))
		os.Exit(1)
	}
}

func connectMessage(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return strings.ToLower(ce.Code().String()) + ": " + ce.Message()
	}
	return err.Error()
}
