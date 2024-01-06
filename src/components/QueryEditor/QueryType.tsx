import { InlineField, RadioButtonGroup } from '@grafana/ui';
import React from 'react';
import { QueryType } from '../../types';
import type { SelectableValue } from '@grafana/data';

const queryTypes: Array<SelectableValue<QueryType>> = [
  { label: 'Metrics', value: QueryType.Metrics, icon: 'chart-line' },
  { label: 'Resource List', value: QueryType.ResourceList, icon: 'table' },
];

interface QueryTypeFieldProps {
  queryType: QueryType;
  onChange: (queryType: QueryType) => void;
}

export function QueryTypeField({ queryType, onChange }: QueryTypeFieldProps) {
  return (
    <InlineField label="Query Type">
      <RadioButtonGroup options={queryTypes} value={queryType} onChange={onChange}></RadioButtonGroup>
    </InlineField>
  );
}
