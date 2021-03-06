package maniladriver

import (
	"bytes"
	"context"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	maniladriverv1alpha1 "github.com/openshift/csi-driver-manila-operator/pkg/apis/maniladriver/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"
)

var (
	manilaSCCManifest = `apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: csi-driver-manila-operator
allowPrivilegedContainer: true
allowPrivilegeEscalation: true
allowHostDirVolumePlugin: true
allowedCapabilities:
- SYS_ADMIN
allowHostIPC: true
allowHostNetwork: true
allowHostPID: false
allowHostPorts: false
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: RunAsAny
fsGroup:
  type: RunAsAny
supplementalGroups:
  type: RunAsAny
users:
- system:serviceaccount:openshift-manila-csi-driver:csi-nodeplugin
- system:serviceaccount:openshift-manila-csi-driver:openstack-manila-csi-controllerplugin
- system:serviceaccount:openshift-manila-csi-driver:openstack-manila-csi-nodeplugin
groups: []
volumes:
- configMap
- downwardAPI
- emptyDir
- hostPath
- nfs
- persistentVolumeClaim
- projected
- secret
`
)

func (r *ReconcileManilaDriver) handleSecurityContextConstraints(instance *maniladriverv1alpha1.ManilaDriver, reqLogger logr.Logger) error {
	reqLogger.Info("Reconciling Manila Security Context Constraints")

	// Define a new Security Context Constraints object
	scc, err := generateSecurityContextConstraints()
	if err != nil {
		return err
	}

	if err := annotator.SetLastAppliedAnnotation(scc); err != nil {
		return err
	}

	// Check if this Security Context Constraints already exists
	found := &securityv1.SecurityContextConstraints{}
	err = r.apiReader.Get(context.TODO(), types.NamespacedName{Name: scc.Name, Namespace: ""}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SecurityContextConstraints", "SecurityContextConstraints.Name", scc.Name)
		err = r.client.Create(context.TODO(), scc)
		if err != nil {
			return err
		}

		// Security Context Constraints created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Check if we need to update the object
	found.TypeMeta = scc.TypeMeta
	equal, err := compareLastAppliedAnnotations(found, scc)
	if err != nil {
		return err
	}

	if !equal {
		reqLogger.Info("Updating SecurityContextConstraints with new changes", "SecurityContextConstraints.Name", found.Name)
		err = r.client.Update(context.TODO(), scc)
		if err != nil {
			return err
		}
	} else {
		// Security Context Constraints already exists - don't requeue
		reqLogger.Info("Skip reconcile: SecurityContextConstraints already exists", "SecurityContextConstraints.Name", found.Name)
	}

	return nil
}

func (r *ReconcileManilaDriver) deleteSecurityContextConstraints(reqLogger logr.Logger) error {
	reqLogger.Info("Deleting Security Context Constraints")

	scc, err := generateSecurityContextConstraints()
	if err != nil {
		return err
	}

	err = r.client.Delete(context.TODO(), scc)
	if err != nil {
		return err
	}

	reqLogger.Info("Security Context Constraints were deleted succesfully", "SecurityContextConstraints.Name", scc.Name)

	return nil
}

func generateSecurityContextConstraints() (*securityv1.SecurityContextConstraints, error) {
	scc := &securityv1.SecurityContextConstraints{}

	dec := k8sYaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manilaSCCManifest)), 1000)
	if err := dec.Decode(scc); err != nil {
		return nil, err
	}

	return scc, nil
}
