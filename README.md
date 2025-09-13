# Hetzner Cloud data source for Grafana

![Example Query](https://github.com/apricote/grafana-hcloud-datasource/raw/main/src/img/screenshot-query.png)

![Version in Marketplace](https://img.shields.io/badge/dynamic/json?logo=grafana&query=$.version&url=https://grafana.com/api/plugins/apricote-hcloud-datasource&label=Marketplace&prefix=v&color=F47A20)
![Downloads in Marketplace](https://img.shields.io/badge/dynamic/json?logo=grafana&query=$.downloads&url=https://grafana.com/api/plugins/apricote-hcloud-datasource&label=Downloads&color=F47A20)
![Grafana Dependency](https://img.shields.io/badge/dynamic/json?logo=grafana&query=$.grafanaDependency&url=https://grafana.com/api/plugins/apricote-hcloud-datasource&label=Grafana&color=F47A20)

## Introduction

With this plugin you can display metrics data for your Hetzner Cloud Servers & Load Balancers in your Grafana dashboards.
It works directly with the Hetzner Cloud API, and does not require Prometheus or any other additional software.

## Requirements

- Grafana 12+

## Getting Started

After you have installed the data source plugin, you need to add a new data source in Grafana. Create a new `read` API Token for the project and set it in the data source settings.

To quickly get started, you can import the included dashboard, which displays all available metrics for servers & load balancers. This is available in a tab on the data source settings page. Alternatively you can also get the JSON from [the repo](https://github.com/apricote/grafana-hcloud-datasource/tree/main/src/dashboards/demo.json).

## Documentation

### Query Editor

![Query Editor](https://github.com/apricote/grafana-hcloud-datasource/raw/main/src/img/screenshot-query-editor.png)

The Query Editor can be hard to grasp at first, so here is an overview of the available options.

#### Selecting Resources

In the query editor, you can choose which resources type, server or load balancer, you want to get metrics for.

To define which servers/load balancers you want to see in the graph, you can choose between the following options:

- **IDs**: A drop-down list of all available servers/load balancers in the project. You can select multiple IDs.
- **Labels**: You can set [label selectors](https://docs.hetzner.cloud/#label-selector) to filter the resources. This is useful if you have a dynamic list of resources.
- **Variable**: This option exists to support using Dashboard-wide variables to select the resources. Should include the `$` prefix of the variable, e.g. `$servers`. See _Using Variables_ for more details.

#### Legend Format

You can rename the returned series names by using the `Legend Format` field in the query editor. This works similar to the Prometheus data source.

In the specified format, you can include the names of labels in `{{ }}` brackets, and they will be replaced with the actual label values.

Right now, the following labels are available:

- `name`: The name of the resource (server or load balancer)
- `id`: The ID of the resource
- `series_name`: Name of the series from the API (e.g. `disk.0.iops.read`)
- `series_display_name`: A human-readable name for the series (e.g. `Read`)

If not specified, the default format is: `{{ series_display_name }} {{ name }}`.

#### Query Type

By default, queries return metrics. It is also possible to select the Query Type **List Resources**. This will return a table of the matching resources with some interesting fields, like the server type and the labels.

The returned field `var` is necessary for _Using Variables_.

#### Using Variables

If you would like to have a dropdown list of servers or load balancers in your dashboard, you can use the `List Resources` query type to get a list of resources.

The returned values from this will look like `$resource_name : $resource_id`. Add the regex `(?<text>.*) : (?<value>.*)` to get the resource name as options.
This regex is also required so you can use the variable in later queries, as it will make the ID the value when the option is selected.

Assuming you created the variable `$servers` with the mentioned regex, you can now create a new panel with the `Metrics` query type, click **Select By Variable** and set the variable name field to `$servers`.

The panel should then automatically update when you select different servers in the dropdown.

You can also take a look at the included dashboard to see in practice how this should be set up.

### Multiple Projects

If you want to access metrics from multiple Hetzner Cloud projects, you need to create a new data source for each
project, with separate API Tokens. The default dashboard has a variable to select the current project.


## Contributing

If you have any questions, feedback or ideas, feel free to open an issue or pull request.

## Support Disclaimer

This is not an official Hetzner Cloud product in any way and Hetzner Cloud does not provide support for this.
