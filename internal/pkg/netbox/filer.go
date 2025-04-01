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
}

func (c Client) GetFilers(ctx context.Context, region, query string) (filers []Filer, err error) {
	switch query {
	case "md", "manila":
		filers, err = c.getNetAppFilers(ctx, region, "manila")
	case "bb", "cinder":
		filers, err = c.getNetAppFilers(ctx, region, "cinder")
	case "bm", "baremetal":
		filers, err = c.getNetAppFilers(ctx, region, "baremetal")
	case "apod", "cp", "control-plane", "control_plane":
		filers, err = c.getNetAppFilers(ctx, region, "apod")
	default:
		err = fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return nil, err
	}
	return filers, nil
}
