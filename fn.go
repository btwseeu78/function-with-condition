package main

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/crossplane/function-with-condition/input/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"slices"
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
	log.Debug("Check Observed Resource", "DR", desired)
	for _, obj := range input.Cfg.Objs {
		cd, _ := observed[resource.Name(obj.Name)]
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

			err := f.patchFieldValueToObject(obj.SourceFieldPath, obj.DestinationFieldPath, obj.MatchValue, obj.FieldValue, obj.Condition, desired[resource.Name(obj.Name)].Resource)

			if err != nil {
				response.Fatal(rsp, errors.Wrap(err, "Unable to patch the object"))
				return rsp, nil
			}
		}
	}
	f.log.Info("Details of obj", "obs", observed, "desired", desired)
	err = response.SetDesiredComposedResources(rsp, desired)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "Unable to generate the desired compose the object"))
		return rsp, err
	}

	return rsp, nil
}

func (f *Function) patchFieldValueToObject(sfp string, dsp string, svalue string, dvalue string, conditon string, to runtime.Object) error {
	log := f.log.WithValues("function", "patchFieldValeu", "path", sfp)
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
		if svalue == "" {
			return errors.New("The Source Field Is Blank")
		}
	case "NotExists":
		if svalue == "" {
			err := paved.SetValue(dsp, dvalue)
			if err != nil {
				return err
			}
		}
	case "Equals":
		if svalue == "" && dvalue == "" {
			return errors.New("You can Do Equality Between Null Values")
		} else {
			fieldval, err := paved.GetString(sfp)
			if err != nil {
				return errors.New("Unable to get value for equality")
			}
			if fieldval == svalue {
				err := paved.SetValue(dsp, dvalue)
				if err != nil {
					return err
				}
			} else {
				log.Debug("The Values Are Not Equal")
			}
		}
	case "NotEqual":
		if svalue == "" && dvalue == "" {
			return errors.New("You can Do NotEquality Between Null Values")
		} else {
			fieldVal, err := paved.GetString(sfp)
			if err != nil {
				return errors.New("Unable to get Value of SourceField")
			}
			if fieldVal != svalue {
				err := paved.SetValue(dsp, dvalue)
				if err != nil {
					return err
				}
			} else {
				log.Info("The Condition Does not Match")
			}

		}
	case "In":
		if svalue == "" {
			log.Debug("Unable to get the Object")
		} else {
			listVal, err := paved.GetStringArray(sfp)
			if err != nil {
				log.Debug("Unable to generate required paved object")
			}

			if slices.Contains(listVal, svalue) {
				err := paved.SetValue(dsp, dvalue)
				if err != nil {
					return err
				}
			} else {

			}

		}

	case "NotIn":
		if svalue == "" {
			log.Debug("The Value is blank Sadly")
		} else {
			listVal, err := paved.GetValue(sfp)
			if err != nil {
				//tmpType := make([]interface{}, 0)
				//cnvrtVal := listVal.([]interface{})
				//tmpType = append(tmpType, cnvrtVal...)
				for val := range listVal {
					log.Info("value after the conversion inside not in is  ::::", val)
				}
			} else {
				log.Info("Errored Out", sfp)
				return err
			}
		}

	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}
