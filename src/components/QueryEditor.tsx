import React from 'react';
import { InlineField, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { DataSourceOptions, Query } from '../types';

type Props = QueryEditorProps<DataSource, Query, DataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onResourceTypeChange = (event: SelectableValue<Query['resourceType']>) => {
    onChange({ ...query, resourceType: event.value! });
    onRunQuery();
  };

  const onQueryTypeChange = (event: SelectableValue<Query['queryType']>) => {
    onChange({ ...query, queryType: event.value! });
    onRunQuery();
  };

  const { queryType, resourceType } = query;

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
    </div>
  );
}
