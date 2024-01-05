import React, { useCallback, useState } from 'react';
import {
  AsyncMultiSelect,
  AutoSizeInput,
  InlineField,
  InlineFieldRow,
  Input,
  LinkButton,
  RadioButtonGroup,
  Select,
  TagsInput,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import {
  DataSourceOptions,
  LoadBalancerMetricsTypes,
  Query,
  QueryType,
  ResourceType,
  SelectBy,
  ServerMetricsTypes,
} from '../types';
import { isValidOption } from '../img/enum';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptions>;

const selectOptionsServerMetricsTypes = [
  { value: ServerMetricsTypes.CPU, label: 'CPU' },
  { value: ServerMetricsTypes.DiskBandwidth, label: 'Disk Bandwidth' },
  { value: ServerMetricsTypes.DiskIOPS, label: 'Disk IOPS' },
  { value: ServerMetricsTypes.NetworkBandwidth, label: 'Network Bandwidth' },
  { value: ServerMetricsTypes.NetworkPPS, label: 'Network PPS' },
];

const selectOptionsLoadBalancerMetricsTypes = [
  { value: LoadBalancerMetricsTypes.OpenConnections, label: 'Open Connections' },
  { value: LoadBalancerMetricsTypes.ConnectionsPerSecond, label: 'Connections Per Second' },
  { value: LoadBalancerMetricsTypes.RequestsPerSecond, label: 'Requests Per Second' },
  { value: LoadBalancerMetricsTypes.Bandwidth, label: 'Bandwidth' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    queryType,
    resourceType,
    metricsType,
    selectBy,
    labelSelectors = [],
    resourceIDs = [],
    resourceIDsVariable = '',
    legendFormat = '',
  } = query;

  const onResourceTypeChange = (event: SelectableValue<Query['resourceType']>) => {
    const resourceType = event.value!;
    let metricsType = query.metricsType;

    // Make sure that the metrics type is valid for the new resource type
    switch (resourceType) {
      case ResourceType.Server: {
        if (!isValidOption(ServerMetricsTypes, metricsType)) {
          metricsType = ServerMetricsTypes.CPU;
        }
        break;
      }
      case ResourceType.LoadBalancer: {
        if (!isValidOption(LoadBalancerMetricsTypes, metricsType)) {
          metricsType = LoadBalancerMetricsTypes.Bandwidth;
        }
      }
    }
    onChange({ ...query, resourceType, metricsType, resourceIDs: [] });
    setFormResourceIDs([]);
    onRunQuery();
  };

  const [formResourceIDs, setFormResourceIDs] = useState<Array<SelectableValue<number>>>(
    resourceIDs.map((id) => ({ value: id }))
  );

  const onResourceNameOrIDsChange = (newValues: Array<SelectableValue<number>>) => {
    onChange({ ...query, resourceIDs: newValues.map((value) => value.value!) });
    onRunQuery();
    setFormResourceIDs(newValues);
  };

  const multiselectLoadResources = useCallback(
    async (_: string) => {
      let resources: Array<SelectableValue<number>> = [];
      switch (resourceType) {
        case ResourceType.Server: {
          resources = await datasource.getServers();
          break;
        }
        case ResourceType.LoadBalancer: {
          resources = await datasource.getLoadBalancers();
          break;
        }
      }

      // In case the QueryEditor was loaded with some resources preselected, this only restored their IDs and the fields look weird.
      // We need to update the formResourceIDs to match the resources we just loaded.

      const hydratedValues = formResourceIDs
        .map(({ value }) => resources.find((r) => r.value === value))
        .filter((r) => r !== undefined);

      setFormResourceIDs(hydratedValues as Array<SelectableValue<number>>);

      return resources;
    },
    [datasource, formResourceIDs, resourceType]
  );

  const availableMetricTypes =
    query.resourceType === 'server' ? selectOptionsServerMetricsTypes : selectOptionsLoadBalancerMetricsTypes;

  return (
    <>
      <InlineFieldRow>
        <InlineField label="Resource Type">
          <Select
            options={[
              { label: 'Server', value: ResourceType.Server },
              { label: 'Load Balancer', value: ResourceType.LoadBalancer },
            ]}
            value={resourceType}
            onChange={onResourceTypeChange}
          ></Select>
        </InlineField>
        {queryType === QueryType.ResourceList && (
          <LabelSelectorInput
            values={labelSelectors}
            onChange={(v) => {
              onChange({ ...query, labelSelectors: v });
              onRunQuery();
            }}
          />
        )}
        {queryType === QueryType.Metrics && (
          <>
            <InlineField label="Metrics Type">
              <Select
                options={availableMetricTypes}
                value={metricsType}
                onChange={(event: SelectableValue<Query['metricsType']>) => {
                  onChange({ ...query, metricsType: event.value! });
                  onRunQuery();
                }}
              ></Select>
            </InlineField>
            <InlineField label={'Select By'}>
              <RadioButtonGroup
                value={selectBy}
                onChange={(v: Query['selectBy']) => {
                  onChange({ ...query, selectBy: v });
                  onRunQuery();
                }}
                options={[
                  { label: 'Labels', value: SelectBy.Label, icon: 'filter' },
                  { label: 'IDs', value: SelectBy.ID, icon: 'gf-layout-simple' },
                  { label: 'Variable', value: SelectBy.Name, icon: 'grafana' },
                ]}
              />
            </InlineField>

            {selectBy === SelectBy.Label && (
              <LabelSelectorInput
                values={labelSelectors}
                onChange={(v) => {
                  onChange({ ...query, labelSelectors: v });
                  onRunQuery();
                }}
              />
            )}
            {selectBy === SelectBy.ID && (
              <InlineField label={resourceType === ResourceType.Server ? 'Servers' : 'Load Balancers'}>
                <AsyncMultiSelect
                  key={resourceType} // Force reloading options when the key changes
                  loadOptions={multiselectLoadResources}
                  value={formResourceIDs}
                  onChange={onResourceNameOrIDsChange}
                  defaultOptions
                  isSearchable={false} // Currently not implemented in loadResources + API methods
                ></AsyncMultiSelect>
              </InlineField>
            )}
            {selectBy === SelectBy.Name && (
              <InlineField label={'Variable Name'} tooltip={'Make sure to prefix with $'}>
                <Input
                  value={resourceIDsVariable}
                  placeholder={'$variableName'}
                  onChange={(e) => onChange({ ...query, resourceIDsVariable: e.currentTarget.value })}
                ></Input>
              </InlineField>
            )}
          </>
        )}
      </InlineFieldRow>

      <InlineFieldRow>
        <InlineField label="Query Type">
          <RadioButtonGroup
            options={[
              { label: 'Metrics', value: QueryType.Metrics },
              { label: 'Resource List', value: QueryType.ResourceList },
            ]}
            value={queryType}
            onChange={(queryType: QueryType) => {
              onChange({ ...query, queryType });
              onRunQuery();
            }}
          ></RadioButtonGroup>
        </InlineField>
        <InlineField
          label={'Legend'}
          tooltip={
            <>
              Series name override or template. Ex. <code>{'{{ name }}'}</code> will be replaced with label value for
              name. Defaults to <code>{'{{series_display_name }} {{ name }}'}</code>. Available labels are:{' '}
              <code>id</code>, <code>name</code>, <code>series_name</code>, <code>series_display_name</code>
            </>
          }
        >
          <AutoSizeInput
            value={legendFormat}
            placeholder={'Auto'}
            minLength={22}
            onCommitChange={(e) => {
              onChange({ ...query, legendFormat: e.currentTarget.value });
              onRunQuery();
            }}
          ></AutoSizeInput>
        </InlineField>
      </InlineFieldRow>
    </>
  );
}

interface LabelSelectorInputProps {
  values: string[];
  onChange: (values: string[]) => void;
}
const LabelSelectorInput = ({ values, onChange }: LabelSelectorInputProps) => (
  <InlineField
    label={'Label Selectors'}
    tooltip={
      <LinkButton
        href="https://docs.hetzner.cloud/#label-selector"
        size="sm"
        variant="secondary"
        icon="external-link-alt"
        target="_blank"
      >
        Docs
      </LinkButton>
    }
    interactive={true} // So user can click the link
  >
    <TagsInput tags={values} onChange={onChange} placeholder={'Selectors (enter key to add)'} />
  </InlineField>
);
