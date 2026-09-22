import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, InputNumber, Modal, Tooltip, message } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useFail2banStatusQuery, getLimitIpNotice } from '@/api/queries/useFail2banStatusQuery';
import {
  type ClientDefaults,
  EMPTY_CLIENT_DEFAULTS,
  writeClientDefaults,
} from '@/lib/clients/default-limits';

/**
 * Sets the IP-limit and HWID-limit that every newly created client inherits.
 * Opened from the Clients actions rail. Mirrors the create dialogs: the HWID field
 * is a plain FormField, the IP field is a Form.Item so the fail2ban notice and the
 * disabled state behave exactly as they do on the client forms.
 */

interface ClientDefaultsModalProps {
  open: boolean;
  /** Current values, read by the page when the dialog opens. */
  current: ClientDefaults;
  onOpenChange: (open: boolean) => void;
  /** Called after a save so the page can react to the new values. */
  onSaved?: (next: ClientDefaults) => void;
}

export default function ClientDefaultsModal({
  open,
  current,
  onOpenChange,
  onSaved,
}: ClientDefaultsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<ClientDefaults>({ defaultValues: EMPTY_CLIENT_DEFAULTS });
  const fail2ban = useFail2banStatusQuery();
  const limitIp = useWatch({ control: methods.control, name: 'limitIp' });

  const limitIpDisabled = !fail2ban.usable;
  const limitIpNotice = getLimitIpNotice(fail2ban, t);

  // Re-seed on every open so a cancelled edit is discarded.
  useEffect(() => {
    if (open) methods.reset(current);
  }, [open, current, methods]);

  function save() {
    const next = writeClientDefaults({
      limitIp: methods.getValues('limitIp'),
      limitHwid: methods.getValues('limitHwid'),
    });
    methods.reset(next);
    onSaved?.(next);
    messageApi.success(t('pages.clients.defaultLimitsSaved'));
    onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.defaultLimitsTitle')}
        okText={t('save')}
        cancelText={t('cancel')}
        onOk={save}
        onCancel={() => onOpenChange(false)}
        destroyOnHidden
      >
        <p className="client-defaults-hint">{t('pages.clients.defaultLimitsHint')}</p>
        <FormProvider {...methods}>
          <Form layout="vertical" onSubmitCapture={(e) => e.preventDefault()}>
            <FormField
              name="limitHwid"
              label={t('pages.clients.limitHwid')}
              tooltip={t('pages.clients.limitHwidDesc')}
              transform={{ output: (v) => Number(v) || 0 }}
            >
              <InputNumber min={0} style={{ width: '100%' }} />
            </FormField>

            <Form.Item label={t('pages.clients.limitIp')}>
              <Tooltip title={limitIpNotice || undefined}>
                <span style={{ display: 'inline-flex', width: '100%' }}>
                  <InputNumber
                    value={limitIp}
                    min={0}
                    disabled={limitIpDisabled}
                    style={
                      limitIpDisabled ? { pointerEvents: 'none', width: '100%' } : { width: '100%' }
                    }
                    onChange={(v) => methods.setValue('limitIp', Number(v) || 0)}
                  />
                </span>
              </Tooltip>
            </Form.Item>
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
