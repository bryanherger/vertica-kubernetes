/*
 (c) Copyright [2021] Micro Focus or one of its affiliates.
 Licensed under the Apache License, Version 2.0 (the "License");
 You may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	vapi "github.com/vertica/vertica-kubernetes/api/v1beta1"
	"github.com/vertica/vertica-kubernetes/pkg/cmds"
	"github.com/vertica/vertica-kubernetes/pkg/events"
	"github.com/vertica/vertica-kubernetes/pkg/names"
	"github.com/vertica/vertica-kubernetes/pkg/paths"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// This is a file that we run with the create_db to run custome SQL. This is
	// passed with the --sql parameter when running create_db.
	PostDBCreateSQLFile = "/home/dbadmin/post-db-create.sql"
)

// CreateDBReconciler will create a database if one wasn't created yet.
type CreateDBReconciler struct {
	VRec    *VerticaDBReconciler
	Log     logr.Logger
	Vdb     *vapi.VerticaDB // Vdb is the CRD we are acting on.
	PRunner cmds.PodRunner
	PFacts  *PodFacts
}

// MakeCreateDBReconciler will build a CreateDBReconciler object
func MakeCreateDBReconciler(vdbrecon *VerticaDBReconciler, log logr.Logger,
	vdb *vapi.VerticaDB, prunner cmds.PodRunner, pfacts *PodFacts) ReconcileActor {
	return &CreateDBReconciler{VRec: vdbrecon, Log: log, Vdb: vdb, PRunner: prunner, PFacts: pfacts}
}

// Reconcile will ensure a DB exists and create one if it doesn't
func (c *CreateDBReconciler) Reconcile(ctx context.Context, req *ctrl.Request) (ctrl.Result, error) {
	// Skip this reconciler entirely if the init policy is not to create the DB.
	if c.Vdb.Spec.InitPolicy != vapi.CommunalInitPolicyCreate {
		return ctrl.Result{}, nil
	}

	// The remaining create_db logic is driven from GenericDatabaseInitializer.
	// This exists to creation an abstraction that is common with revive_db.
	g := GenericDatabaseInitializer{
		initializer: c,
		VRec:        c.VRec,
		Log:         c.Log,
		Vdb:         c.Vdb,
		PRunner:     c.PRunner,
		PFacts:      c.PFacts,
	}
	return g.checkAndRunInit(ctx)
}

// execCmd will do the actual execution of admintools -t create_db.
// This handles logging of necessary events.
func (c *CreateDBReconciler) execCmd(ctx context.Context, atPod types.NamespacedName, cmd []string) (ctrl.Result, error) {
	c.VRec.EVRec.Event(c.Vdb, corev1.EventTypeNormal, events.CreateDBStart,
		"Calling 'admintools -t create_db'")
	start := time.Now()
	stdout, _, err := c.PRunner.ExecAdmintools(ctx, atPod, ServerContainer, cmd...)
	if err != nil {
		switch {
		case isEndpointBadError(stdout):
			c.VRec.EVRec.Eventf(c.Vdb, corev1.EventTypeWarning, events.S3EndpointIssue,
				"Unable to write to the bucket in the S3 endpoint '%s'", c.Vdb.Spec.Communal.Endpoint)
			return ctrl.Result{Requeue: true}, nil

		case isBucketNotExistError(stdout):
			c.VRec.EVRec.Eventf(c.Vdb, corev1.EventTypeWarning, events.S3BucketDoesNotExist,
				"The bucket in the S3 path '%s' does not exist", paths.GetCommunalPath(c.Vdb))
			return ctrl.Result{Requeue: true}, nil

		case isCommunalPathNotEmpty(stdout):
			c.VRec.EVRec.Eventf(c.Vdb, corev1.EventTypeWarning, events.CommunalPathIsNotEmpty,
				"The communal path '%s' is not empty", paths.GetCommunalPath(c.Vdb))
			return ctrl.Result{Requeue: true}, nil

		default:
			c.VRec.EVRec.Event(c.Vdb, corev1.EventTypeWarning, events.CreateDBFailed,
				"Failed to create the database")
			return ctrl.Result{}, err
		}
	}
	c.VRec.EVRec.Eventf(c.Vdb, corev1.EventTypeNormal, events.CreateDBSucceeded,
		"Successfully created database with subcluster '%s'. It took %s", c.Vdb.Spec.Subclusters[0].Name, time.Since(start))
	return ctrl.Result{}, nil
}

func isCommunalPathNotEmpty(op string) bool {
	re := regexp.MustCompile(`Communal location \[.+\] is not empty`)
	return re.FindAllString(op, -1) != nil
}

// preCmdSetup will generate the file we include with the create_db.
// This file runs any custom SQL for the create_db.
func (c *CreateDBReconciler) preCmdSetup(ctx context.Context, atPod types.NamespacedName) error {
	// We include SQL to reset the AWS connection parms we temporarily set in the
	// auth file (see constructAuthParms).  We also rename the default
	// subcluster to match the name of the first subcluster in the spec -- any
	// remaining subclusters will be added by DBAddSubclusterReconciler.
	sql := "alter database default clear AWSConnectTimeout;\n" +
		"alter database default clear AWSMaxRetryCount;\n" +
		"alter subcluster default_subcluster rename to " + c.Vdb.Spec.Subclusters[0].Name + ";\n"
	if c.Vdb.Spec.KSafety == vapi.KSafety0 {
		sql += "select set_preferred_ksafe(0);\n"
	}
	_, _, err := c.PRunner.ExecInPod(ctx, atPod, ServerContainer,
		"bash", "-c", "cat > "+PostDBCreateSQLFile+"<<< '"+sql+"'",
	)
	return err
}

// getAdditionalAuthParms returns additional auth parms that we need to set for create_db
func (c *CreateDBReconciler) getAdditionalAuthParms() string {
	// We temporarily lower the connect time and retry count for AWS. This is
	// done so that we fail fast if the S3 endpoint isn't setup. These are
	// cleared at the end of the create_db.
	const TempAWSConnectTime = "20"
	const TempMaxRetryCount = "3"

	return fmt.Sprintf("%s = %s\n%s = %s\n",
		"AWSConnectTimeout", TempAWSConnectTime,
		"AWSMaxRetryCount", TempMaxRetryCount,
	)
}

// getPodList gets a list of all of the pods we are going to use with create db.
// If any pod is not found in the pod facts, it return false for the bool
// return value.
func (c *CreateDBReconciler) getPodList() ([]*PodFact, bool) {
	// We grab all pods from the first subcluster.  Pods for additional
	// subcluster are added through db_add_node.
	sc := &c.Vdb.Spec.Subclusters[0]
	podList := make([]*PodFact, 0, sc.Size)
	for i := int32(0); i < sc.Size; i++ {
		pn := names.GenPodName(c.Vdb, sc, i)
		pf, ok := c.PFacts.Detail[pn]
		// Bail out if one of the pods in the subcluster isn't found
		if !ok {
			return []*PodFact{}, false
		}
		podList = append(podList, pf)
	}
	// We need the podList to be ordered by its compat21 node number. This
	// ensures the assigned vnode number will match the compat21 node number.
	// admintools -t restart_db depends on this.
	sort.Slice(podList, func(i, j int) bool {
		return podList[i].compat21NodeName < podList[j].compat21NodeName
	})

	// In case that kSafety == 0 (KSafety0), we only pick one pod from the first subcluster
	// The remaining pods would be added with db_add_node.
	if c.Vdb.Spec.KSafety == vapi.KSafety0 {
		return podList[0:1], true
	}
	// Otherwise, we pick all pods from the first subcluster
	return podList, true
}

// genCmd will return the command to run in the pod to create the database
func (c *CreateDBReconciler) genCmd(hostList []string) []string {
	return []string{
		"-t", "create_db",
		"--skip-fs-checks",
		"--hosts=" + strings.Join(hostList, ","),
		"--communal-storage-location=" + paths.GetCommunalPath(c.Vdb),
		"--communal-storage-params=" + paths.AuthParmsFile,
		"--sql=" + PostDBCreateSQLFile,
		fmt.Sprintf("--shard-count=%d", c.Vdb.Spec.ShardCount),
		"--depot-path=" + c.Vdb.Spec.Local.DepotPath,
		"--database", c.Vdb.Spec.DBName,
		"--force-cleanup-on-failure",
		"--noprompt",
	}
}
