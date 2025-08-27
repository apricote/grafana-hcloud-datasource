import { AutoSizeInput, InlineField } from '@grafana/ui';
import React from 'react';

const LABELS = ['id', 'name', 'series_name', 'series_display_name'];

interface LegendFormatFieldProps {
  legendFormat: string;
  onChange: (legendFormat: string) => void;
}
export function LegendFormatField({ legendFormat, onChange }: LegendFormatFieldProps) {
  return (
    <InlineField
      label={'Legend'}
      tooltip={
        <>
          Series name override or template. Ex. <code>{'{{ name }}'}</code> will be replaced with label value for name.
          Defaults to <code>{'{{series_display_name }} {{ name }}'}</code>. Available labels are:{' '}
          {LABELS.map<React.ReactNode>((label, i) => <code key={i}>{label}</code>).reduce((prev, cur) => [
            prev,
            ', ',
            cur,
          ])}
        </>
      }
    >
      <AutoSizeInput
        value={legendFormat}
        placeholder={'Auto'}
        minLength={22}
        onChange={(e) => onChange(e.currentTarget.value)}
      ></AutoSizeInput>
    </InlineField>
  );
}
