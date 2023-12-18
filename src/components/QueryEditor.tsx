import React from 'react';
import { InlineField, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { DataSourceOptions, LoadBalancerMetricsTypes, Query, ServerMetricsTypes } from '../types';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
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
    onChange({ ...query, resourceType, metricsType });
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

  const availableMetricTypes = query.resourceType === 'server' ? ServerMetricsTypes : LoadBalancerMetricsTypes;

  const { queryType, resourceType, metricsType } = query;

  return (
    <div className="gf-form">
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
        <InlineField label="Metrics Type">
          <Select
            options={availableMetricTypes.map((type) => ({ label: type, value: type }))}
            value={metricsType}
            onChange={onMetricsTypeChange}
          ></Select>
        </InlineField>
      )}
    </div>
  );
}
