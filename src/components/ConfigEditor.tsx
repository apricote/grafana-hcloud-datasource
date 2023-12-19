import React, { ChangeEvent } from 'react';
import { Badge, Checkbox, Field, FieldSet, Icon, LinkButton, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { DataSourceOptions, SecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<DataSourceOptions> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;

  // Secure field (only sent to the backend)
  const onAPITokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        apiToken: event.target.value,
      },
    });
  };

  const onResetAPIToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        apiToken: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        apiToken: '',
      },
    });
  };

  const onDebugChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        debug: event.target.checked,
      },
    });
  };

  const { secureJsonFields } = options;
  const secureJsonData = (options.secureJsonData || {}) as SecureJsonData;
  const jsonData = options.jsonData;

  return (
    <div className="gf-form-group">
      <Field
        label="API Token"
        required
        description={
          <>
            You can create the token at{' '}
            <LinkButton
              href="https://console.hetzner.cloud/projects"
              size="sm"
              variant="secondary"
              icon="external-link-alt"
              target="_blank"
            >
              console.hetzner.cloud
            </LinkButton>
            .
            <br /> Select a project and navigate to{' '}
            <Badge
              text={
                <>
                  Security <Icon name="angle-right" /> API tokens <Icon name="angle-right" /> Generate API token
                </>
              }
              color="blue"
            />
          </>
        }
      >
        <SecretInput
          isConfigured={(secureJsonFields && secureJsonFields.apiToken) as boolean}
          value={secureJsonData.apiToken || ''}
          placeholder="The read-only API Token for your Project"
          width={48}
          onReset={onResetAPIToken}
          onChange={onAPITokenChange}
        />
      </Field>
      <FieldSet label="Development">
        <p>These option are used to develop the Datasource. It should not be necessary to set them in production.</p>
        <Checkbox
          value={jsonData.debug}
          label={'Debug Logging'}
          description={'Enable to see all requests & responses with the Hetzner Cloud API in the Grafana Logs'}
          onChange={onDebugChange}
        ></Checkbox>
      </FieldSet>
    </div>
  );
}
