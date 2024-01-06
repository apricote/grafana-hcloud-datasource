import { InlineField, RadioButtonGroup } from '@grafana/ui';
import React from 'react';
import { QueryType } from '../../types';

const queryTypes = [
  { label: 'Metrics', value: QueryType.Metrics },
  { label: 'Resource List', value: QueryType.ResourceList },
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
