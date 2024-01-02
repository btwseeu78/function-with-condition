package main

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/function-sdk-go/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane/function-with-condition/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	log := f.log.WithValues("tag", req.GetMeta().GetTag())
	log.Info("Running Function")
	rsp := response.To(req, response.DefaultTTL)

	input := &v1beta1.PatchWithCondition{}
	if err := request.GetInput(req, input); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}
	// Need to add validation for later blame and create task :)

	// *** do not forget as this is critical ***

	// Get The Composite Resource

	oxr, err := request.GetObservedCompositeResource(req)

	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "Can not get observed composite resource"))
		return rsp, nil
	}

	log = log.WithValues(
		"xr-version", oxr.Resource.GetAPIVersion(),
		"xr-kind", oxr.Resource.GetKind(),
		"xr-name", oxr.Resource.GetName(),
	)

	// Desired Resource

	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "Can not get desired ComposedResource"))
		return rsp, err
	}

	observed, err := request.GetObservedComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "Can not get observed ComposedResource"))
		return rsp, err
	}

	// *** substitution Loop *** //

	for _, obj := range input.Cfg.Objs {
		cd, ok := observed[resource.Name(obj.Name)]
		if !ok {
			response.Fatal(rsp, errors.Wrap(err, "The Specified Resource doees not exist"))
			return rsp, nil
		}
		if cd.Resource != nil {
			observedPaved, err := fieldpath.PaveObject(cd.Resource)
			if err != nil {
				response.Fatal(rsp, errors.Wrap(err, "Can not create paved object from observed Resource"))
				return rsp, err
			}
			getFieldPath, err := observedPaved.GetValue(obj.DestinationFieldPath)
			if err != nil {
				response.Fatal(rsp, errors.Wrap(err, "Can not get value of fieldpath from observed Resource"))
				return rsp, err
			}
			getFieldValue, err := observedPaved.GetValue(obj.FieldValue)
			if err != nil {
				response.Fatal(rsp, errors.Wrap(err, "Can not get value of fieldValue from observed Resource"))
				return rsp, err
			}

			log.Debug("Found Corresponding Observed resource", "Path", getFieldPath, "Value", getFieldValue)
		}
		if cd.Resource == nil && obj.FieldValue != "" {
			err := patchFieldValueToObject(obj.SourceFieldPath, obj.DestinationFieldPath, obj.SourceFieldValue, obj.FieldValue, obj.Condition, desired[resource.Name(obj.Name)].Resource)

			if err != nil {
				response.Fatal(rsp, errors.Wrap(err, "Unable to patch the object"))
				return rsp, err
			}
		}
	}
	response.SetDesiredComposedResources(rsp, desired)
	return rsp, nil
}

func patchFieldValueToObject(sfp string, dsp string, svalue string, dvalue string, conditon string, to runtime.Object) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}
	switch conditon {
	case "Exists":
		if svalue != "" && sfp != "" {
			err := paved.SetValue(dsp, dvalue)
			if err != nil {
				return err
			}
		}
	case "NotExists":
		if svalue == "" {
			err := paved.SetValue(dsp, dvalue)
			if err != nil {
				return err
			}
		}
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}
