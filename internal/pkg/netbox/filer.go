package netbox

import (
	"context"
	"fmt"
)

type Filer struct {
	Name             string `json:"name" yaml:"name"`
	Host             string `json:"host" yaml:"host"`
	AvailabilityZone string `json:"availability_zone" yaml:"availability_zone"`
	Ip               string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Status           string `json:"status,omitempty" yaml:"status,omitempty"`
	Facility         string `json:"facility,omitempty" yaml:"facility,omitempty"`
	Service          string `json:"service,omitempty" yaml:"service,omitempty"`
}

func (c Client) GetFilers(ctx context.Context, region, query string) (filers []Filer, err error) {
	var service string
	switch query {
	case "md", "manila":
		service = "manila"
		filers, err = c.getNetAppFilers(ctx, region, "manila")
		if err != nil {
			return nil, err
		}
		// Exclude filers that have both cinder and manila tags
		dualTagged, err := c.getFilers(ctx, region, "cinder", "manila")
		if err != nil {
			return nil, err
		}
		filers = excludeFilers(filers, dualTagged)
	case "bb", "cinder":
		service = "cinder"
		filers, err = c.getNetAppFilers(ctx, region, "cinder")
		if err != nil {
			return nil, err
		}
		// Exclude filers that have both cinder and manila tags
		dualTagged, err := c.getFilers(ctx, region, "cinder", "manila")
		if err != nil {
			return nil, err
		}
		filers = excludeFilers(filers, dualTagged)
	case "cm", "cinder-manila", "cinder_manila":
		service = "cinder_manila"
		filers, err = c.getFilers(ctx, region, "cinder", "manila")
	case "bm", "baremetal":
		service = "baremetal"
		filers, err = c.getNetAppFilers(ctx, region, "baremetal")
	case "apod", "cp", "control-plane", "control_plane":
		service = "apod"
		filers, err = c.getNetAppFilers(ctx, region, "apod")
	default:
		err = fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return nil, err
	}
	for i := range filers {
		filers[i].Service = service
	}
	return filers, nil
}

// excludeFilers removes filers from the list that are present in the exclude list.
func excludeFilers(filers, exclude []Filer) []Filer {
	excludeMap := make(map[string]struct{}, len(exclude))
	for _, f := range exclude {
		excludeMap[f.Name] = struct{}{}
	}
	result := make([]Filer, 0, len(filers))
	for _, f := range filers {
		if _, ok := excludeMap[f.Name]; !ok {
			result = append(result, f)
		}
	}
	return result
}
