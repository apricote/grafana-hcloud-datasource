import React, { useCallback, useState } from 'react';
import { AsyncMultiSelect, InlineField, InlineFieldRow, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { DataSourceOptions, LoadBalancerMetricsTypes, Query, ServerMetricsTypes } from '../types';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const onResourceTypeChange = (event: SelectableValue<Query['resourceType']>) => {
    const resourceType = event.value!;
    let metricsType = query.metricsType;

    // Make sure that the metrics type is valid for the new resource type
    switch (resourceType) {
      case 'server': {
        if (!ServerMetricsTypes.includes(metricsType as any)) {
          metricsType = 'cpu';
        }
        break;
      }
      case 'load-balancer': {
        if (!LoadBalancerMetricsTypes.includes(metricsType as any)) {
          metricsType = 'open-connections';
        }
      }
    }
    onChange({ ...query, resourceType, metricsType, resourceIDs: [] });
    setFormResourceIDs([]);
    onRunQuery();
  };

  const onQueryTypeChange = (event: SelectableValue<Query['queryType']>) => {
    onChange({ ...query, queryType: event.value! });
    onRunQuery();
  };

  const onMetricsTypeChange = (event: SelectableValue<Query['metricsType']>) => {
    onChange({ ...query, metricsType: event.value! });
    onRunQuery();
  };

  const { queryType, resourceType, metricsType, resourceIDs } = query;

  const multiselectLoadResources = useCallback(
    async (_: string) => {
      switch (resourceType) {
        case 'server': {
          return datasource.getServers();
        }
        case 'load-balancer': {
          return datasource.getLoadBalancers();
        }
      }
    },
    [datasource, resourceType]
  );

  // Foobar
  // TODO Properly restore the selected resources after the options are loaded,
  // currently we always show empty form even if the query has IDs set
  const [formResourceIDs, setFormResourceIDs] = useState<Array<SelectableValue<number>>>(
    resourceIDs.map((id) => ({ value: id }))
  );
  const onResourceNameOrIDsChange = (newValues: Array<SelectableValue<number>>) => {
    onChange({ ...query, resourceIDs: newValues.map((value) => value.value!) });
    onRunQuery();
    setFormResourceIDs(newValues);
  };

  const availableMetricTypes = query.resourceType === 'server' ? ServerMetricsTypes : LoadBalancerMetricsTypes;

  return (
    <InlineFieldRow>
      <InlineField label="Query Type">
        <Select
          options={[
            { label: 'Metrics', value: 'metrics' },
            { label: 'Resource List', value: 'resource-list' },
          ]}
          value={queryType}
          onChange={onQueryTypeChange}
        ></Select>
      </InlineField>
      <InlineField label="Resource Type">
        <Select
          options={[
            { label: 'Server', value: 'server' },
            { label: 'Load Balancer', value: 'load-balancer' },
          ]}
          value={resourceType}
          onChange={onResourceTypeChange}
        ></Select>
      </InlineField>
      {queryType === 'metrics' && (
        <>
          <InlineField label="Metrics Type">
            <Select
              options={availableMetricTypes.map((type) => ({ label: type, value: type }))}
              value={metricsType}
              onChange={onMetricsTypeChange}
            ></Select>
          </InlineField>
          <InlineField required label={resourceType === 'server' ? 'Servers' : 'Load Balancers'}>
            <AsyncMultiSelect
              key={resourceType} // Force reloading options when the key changes
              loadOptions={multiselectLoadResources}
              value={formResourceIDs}
              onChange={onResourceNameOrIDsChange}
              defaultOptions
              isSearchable={false} // Currently not implemented in loadResources + API methods
            ></AsyncMultiSelect>
          </InlineField>
        </>
      )}
    </InlineFieldRow>
  );
}
