import React, { ChangeEvent } from 'react';
import { Badge, Checkbox, FieldSet, Icon, InlineField, LinkButton, SecretInput, Stack } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { DataSourceOptions, SecureJsonData } from '../types';
import { OptionGroup } from './OptionGroup';

const EXPECTED_API_TOKEN_LENGTH = 64;

interface Props extends DataSourcePluginOptionsEditorProps<DataSourceOptions> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;

  // Secure field (only sent to the backend)
  const [apiTokenError, setAPITokenError] = React.useState<string | null>(null);
  const onAPITokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    const token = event.target.value;

    if (token.length !== EXPECTED_API_TOKEN_LENGTH) {
      setAPITokenError(`Must be ${EXPECTED_API_TOKEN_LENGTH} characters long`);
    } else {
      setAPITokenError(null);
    }

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

  const collapsedInfoList = [`Debug Logging: ${jsonData.debug ? 'Enabled' : 'Disabled'}`];

  return (
    <div className="gf-form-group">
      <FieldSet label={'Authentication'}>
        <p>
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
        </p>

        <InlineField label="API Token" required invalid={apiTokenError !== null} error={apiTokenError}>
          <SecretInput
            isConfigured={(secureJsonFields && secureJsonFields.apiToken) as boolean}
            value={secureJsonData.apiToken || ''}
            placeholder="The read-only API Token for your Project"
            prefix={<Icon name="lock" />}
            width={EXPECTED_API_TOKEN_LENGTH}
            onReset={onResetAPIToken}
            onChange={onAPITokenChange}
          />
        </InlineField>
      </FieldSet>
      <FieldSet label={'Development'}>
        <p>These option are used to develop the Datasource. It should not be necessary to set them in production.</p>
        <OptionGroup title="Options" collapsedInfo={collapsedInfoList}>
          <Stack direction={'column'}>
            <Checkbox
              value={jsonData.debug}
              label={'Debug Logging'}
              description={'Enable to see all requests & responses with the Hetzner Cloud API in the Grafana Logs'}
              onChange={onDebugChange}
            ></Checkbox>
          </Stack>
        </OptionGroup>
      </FieldSet>
    </div>
  );
}
