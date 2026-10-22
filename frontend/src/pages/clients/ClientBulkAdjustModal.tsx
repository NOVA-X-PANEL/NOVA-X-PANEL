import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Checkbox, Collapse, Form, Input, InputNumber, Modal, Radio, Select, message } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { ClientBulkAdjustFormSchema, type ClientBulkAdjustFormValues } from '@/schemas/client';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives/flow';
import { FormField } from '@/components/form/rhf';

const GB = 1024 * 1024 * 1024;

const FLOW_CLEAR = 'none';

const EMPTY: ClientBulkAdjustFormValues = {
  all: false,
  skipUnlimited: true,
  addDays: 0,
  addGB: 0,
  flow: '',
  limitHwid: null,
  adTag: '',
};

interface ClientBulkAdjustModalProps {
  open: boolean;
  count: number;
  isAll?: boolean;
  totalCount?: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (
    addDays: number,
    addBytes: number,
    flow: string,
    limitHwid?: number | null,
    adTag?: string,
    all?: boolean,
    skipUnlimited?: boolean,
  ) => Promise<{ adjusted: number; skipped?: { email: string; reason: string }[] } | null>;
}

export default function ClientBulkAdjustModal({
  open,
  count,
  isAll = false,
  totalCount = 0,
  onOpenChange,
  onSubmit,
}: ClientBulkAdjustModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [submitting, setSubmitting] = useState(false);
  const [applyTarget, setApplyTarget] = useState<'selected' | 'all'>(isAll || count === 0 ? 'all' : 'selected');
  const [skipUnlimited, setSkipUnlimited] = useState(true);
  const methods = useForm<ClientBulkAdjustFormValues>({ defaultValues: EMPTY });

  useEffect(() => {
    if (open) {
      methods.reset(EMPTY);
      setApplyTarget(isAll || count === 0 ? 'all' : 'selected');
      setSkipUnlimited(true);
    }
  }, [open, isAll, count, methods]);

  async function handleOk() {
    const values = methods.getValues();
    const validated = ClientBulkAdjustFormSchema.safeParse({
      all: applyTarget === 'all',
      skipUnlimited,
      addDays: Math.trunc(Number(values.addDays) || 0),
      addGB: Number(values.addGB) || 0,
      flow: values.flow,
      limitHwid:
        values.limitHwid !== null &&
        values.limitHwid !== undefined &&
        (values.limitHwid as unknown) !== ''
          ? Math.trunc(Number(values.limitHwid))
          : null,
      adTag: values.adTag?.trim() ?? '',
    });
    if (!validated.success) {
      messageApi.warning(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    const { addDays: days, addGB: gb, flow: flowValue, limitHwid, adTag } = validated.data;
    setSubmitting(true);
    try {
      const bytes = Math.trunc(gb * GB);
      const isTargetAll = applyTarget === 'all';
      const result = await onSubmit(days, bytes, flowValue, limitHwid, adTag, isTargetAll, skipUnlimited);
      if (!result) return;
      const ok = result.adjusted ?? 0;
      const skipped = result.skipped?.length ?? 0;
      if (skipped === 0) {
        messageApi.success(t('pages.clients.toasts.bulkAdjusted', { count: ok }));
      } else {
        const firstReason = result.skipped?.[0]?.reason ?? '';
        messageApi.warning(
          firstReason
            ? `${t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped })} — ${firstReason}`
            : t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped }),
        );
      }
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  }

  const modalTitle =
    applyTarget === 'all'
      ? t('pages.clients.bulkAdjustAllTitle')
      : t('pages.clients.bulkAdjustTitle', { count });

  const advancedItems = [
    {
      key: 'advanced',
      label: t('pages.clients.bulkAdvancedSettings'),
      children: (
        <>
          <FormField name="flow" label={t('pages.clients.bulkFlow')}>
            <Select
              style={{ width: '100%' }}
              options={[
                { value: '', label: t('pages.clients.bulkFlowNoChange') },
                { value: FLOW_CLEAR, label: t('pages.clients.bulkFlowDisable') },
                ...Object.values(TLS_FLOW_CONTROL).map((k) => ({ value: k, label: k })),
              ]}
            />
          </FormField>
          <FormField
            name="limitHwid"
            label={t('pages.clients.limitHwid')}
            tooltip={t('pages.clients.limitHwidDesc')}
          >
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              step={1}
              precision={0}
              placeholder={t('pages.clients.bulkFlowNoChange')}
            />
          </FormField>
          <FormField
            name="adTag"
            label={t('pages.clients.mtprotoAdTag')}
            extra={t('pages.clients.bulkAdTagHint')}
          >
            <Input placeholder={t('pages.clients.bulkFlowNoChange')} allowClear />
          </FormField>
        </>
      ),
    },
  ];

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={modalTitle}
        okText={t('apply')}
        cancelText={t('cancel')}
        confirmLoading={submitting}
        onOk={handleOk}
        onCancel={() => onOpenChange(false)}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('pages.clients.bulkAdjustHint')}
        />

        {count > 0 && (
          <div style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6, fontWeight: 500 }}>{t('pages.clients.bulkAdjustTarget')}</div>
            <Radio.Group
              value={applyTarget}
              onChange={(e) => setApplyTarget(e.target.value)}
            >
              <Radio value="selected">{t('pages.clients.bulkAdjustTargetSelected', { count })}</Radio>
              <Radio value="all">{t('pages.clients.bulkAdjustTargetAll', { count: totalCount || t('all') })}</Radio>
            </Radio.Group>
          </div>
        )}

        <FormProvider {...methods}>
          <Form layout="vertical">
            <FormField name="addDays" label={t('pages.clients.addDays')}>
              <InputNumber style={{ width: '100%', direction: 'ltr' }} step={1} precision={0} placeholder="0" />
            </FormField>
            <FormField name="addGB" label={t('pages.clients.addTrafficGB')}>
              <InputNumber style={{ width: '100%', direction: 'ltr' }} step={1} placeholder="0" />
            </FormField>

            <div style={{ marginBottom: 16, marginTop: 8 }}>
              <Checkbox
                checked={skipUnlimited}
                onChange={(e) => setSkipUnlimited(e.target.checked)}
              >
                {t('pages.clients.skipUnlimitedUsers')}
              </Checkbox>
              <div style={{ fontSize: 12, color: 'var(--text-muted, #888)', marginTop: 4, marginInlineStart: 24 }}>
                {t('pages.clients.skipUnlimitedUsersDesc')}
              </div>
            </div>

            <Collapse ghost items={advancedItems} style={{ marginTop: 8 }} />
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
