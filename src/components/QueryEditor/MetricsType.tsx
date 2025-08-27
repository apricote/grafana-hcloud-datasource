import { InlineField, Combobox, ComboboxOption } from '@grafana/ui';
import { LoadBalancerMetricsTypes, MetricsType, ResourceType, ServerMetricsTypes } from '../../types';
import React from 'react';

const serverOptions = [
  { value: ServerMetricsTypes.CPU, label: 'CPU' },
  { value: ServerMetricsTypes.DiskBandwidth, label: 'Disk Bandwidth' },
  { value: ServerMetricsTypes.DiskIOPS, label: 'Disk IOPS' },
  { value: ServerMetricsTypes.NetworkBandwidth, label: 'Network Bandwidth' },
  { value: ServerMetricsTypes.NetworkPPS, label: 'Network PPS' },
];

const lbOptions = [
  { value: LoadBalancerMetricsTypes.OpenConnections, label: 'Open Connections' },
  { value: LoadBalancerMetricsTypes.ConnectionsPerSecond, label: 'Connections Per Second' },
  { value: LoadBalancerMetricsTypes.RequestsPerSecond, label: 'Requests Per Second' },
  { value: LoadBalancerMetricsTypes.Bandwidth, label: 'Bandwidth' },
];

interface MetricsTypeFieldProps {
  metricsType: MetricsType;
  resourceType: ResourceType;
  onChange: (metricsType: MetricsType) => void;
}

export function MetricsTypeField({ metricsType, resourceType, onChange }: MetricsTypeFieldProps) {
  const availableMetricTypes = resourceType === ResourceType.Server ? serverOptions : lbOptions;

  return (
    <InlineField label="Metrics Type">
      <Combobox
        options={availableMetricTypes}
        value={metricsType}
        onChange={(event: ComboboxOption<MetricsType>) => onChange(event.value!)}
      ></Combobox>
    </InlineField>
  );
}
