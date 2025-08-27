import { InlineField, RadioButtonGroup } from '@grafana/ui';
import { ResourceType } from '../../types';
import React from 'react';

const resourceTypes = [
  { label: 'Server', value: ResourceType.Server },
  { label: 'Load Balancer', value: ResourceType.LoadBalancer },
];

interface ResourceTypeFieldProps {
  resourceType: ResourceType;
  onChange: (resourceType: ResourceType) => void;
}

export function ResourceTypeField({ resourceType, onChange }: ResourceTypeFieldProps) {
  return (
    <InlineField label="Resource Type">
      <RadioButtonGroup
        options={resourceTypes}
        value={resourceType}
        onChange={(value) => onChange(value)}
      ></RadioButtonGroup>
    </InlineField>
  );
}
