import { SelectBy } from '../../types';
import { InlineField, RadioButtonGroup } from '@grafana/ui';
import React from 'react';

interface SelectByFieldProps {
  selectBy: SelectBy;
  onChange: (selectBy: SelectBy) => void;
}

const selectByOptions = [
  { label: 'Labels', value: SelectBy.Label, icon: 'filter' },
  { label: 'IDs', value: SelectBy.ID, icon: 'gf-layout-simple' },
  { label: 'Variable', value: SelectBy.Name, icon: 'grafana' },
];

export function SelectByField({ selectBy, onChange }: SelectByFieldProps) {
  return (
    <InlineField label={'Select By'}>
      <RadioButtonGroup value={selectBy} onChange={onChange} options={selectByOptions} />
    </InlineField>
  );
}
