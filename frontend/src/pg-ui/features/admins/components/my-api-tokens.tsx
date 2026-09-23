// @ts-nocheck — pg-ui surface; follows the vendored component conventions.
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, KeyRound, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

import { Badge } from '@/pg-ui/components/ui/badge';
import { Button } from '@/pg-ui/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/pg-ui/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/pg-ui/components/ui/dialog';
import { Input } from '@/pg-ui/components/ui/input';
import { Label } from '@/pg-ui/components/ui/label';
import { Separator } from '@/pg-ui/components/ui/separator';
import { Skeleton } from '@/pg-ui/components/ui/skeleton';
import { Switch } from '@/pg-ui/components/ui/switch';
import { HttpUtil } from '@/utils';

/**
 * Self-service API tokens.
 *
 * Shown to an account the owner has granted API access to. A token created here
 * acts as *this* account: the server checks the account's role on every request,
 * so the token can never do more than the account can do in the panel. The
 * plaintext is displayed once, at creation, and only its hash is stored.
 */

interface ApiTokenRow {
  id: number;
  name: string;
  enabled: boolean;
  scope: string;
  expiresAt: number;
  createdAt: number;
}

/**
 * Reads this account's tokens.
 *
 * GET, matching `g.GET("/apiTokens")`. This was a POST, and the route only
 * accepts GET, so every load returned 404 — and the old `catch` then swallowed it
 * into an empty array. The page said "No tokens yet" forever, with no error,
 * which is why it looked like a dead screen rather than a broken request.
 *
 * The failure is now thrown so React Query can surface it: the list renders the
 * reason instead of an empty state.
 */
async function readList(): Promise<ApiTokenRow[]> {
  const res = await HttpUtil.get('/panel/api/admins/apiTokens', undefined, { silent: true });
  if (res.success === false) {
    throw new Error(res.msg || 'Could not load your tokens');
  }
  return Array.isArray(res?.obj) ? res.obj : [];
}

function formatDate(unixSeconds: number): string {
  if (!unixSeconds) return '—';
  try {
    return new Date(unixSeconds * 1000).toLocaleDateString();
  } catch {
    return '—';
  }
}

export default function MyApiTokens() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [plaintext, setPlaintext] = useState<string | null>(null);

  const tokens = useQuery({ queryKey: ['my-api-tokens'], queryFn: readList });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['my-api-tokens'] });

  const createMutation = useMutation({
    mutationFn: async (tokenName: string) => {
      const res = await HttpUtil.post('/panel/api/admins/apiTokens/create', { name: tokenName, expiresAt: 0 });
      return res?.obj;
    },
    onSuccess: (row) => {
      setPlaintext(row?.token ?? null);
      setCreateOpen(false);
      setName('');
      refresh();
    },
    onError: () => toast.error(t('admins.apiTokenCreateFailed', { defaultValue: 'Could not create the token' })),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/admins/apiTokens/delete/${id}`),
    onSuccess: refresh,
    onError: () => toast.error(t('admins.apiTokenDeleteFailed', { defaultValue: 'Could not delete the token' })),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      HttpUtil.post(`/panel/api/admins/apiTokens/setEnabled/${id}`, { enabled }),
    onSuccess: refresh,
  });

  const rows = tokens.data ?? [];

  return (
    <div className="flex flex-col gap-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="h-4 w-4" />
              {t('admins.myApiTitle', { defaultValue: 'My API token' })}
            </CardTitle>
            <CardDescription>
              {t('admins.myApiDescription', {
                defaultValue:
                  'Create a token that acts as your account. It carries your role, so it can do exactly what you can do here — nothing more. Keep it secret; the value is shown only once.',
              })}
            </CardDescription>
          </div>
          <Button onClick={() => setCreateOpen(true)} size="sm">
            <Plus className="mr-1 h-4 w-4" />
            {t('admins.apiTokenCreate', { defaultValue: 'Create token' })}
          </Button>
        </CardHeader>
        <CardContent>
          {tokens.isLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : tokens.isError ? (
            // Without this branch a failed request looks identical to an empty
            // list — which is how the POST/GET mismatch stayed invisible while
            // the page showed "No tokens yet." forever.
            <p className="text-destructive text-sm">
              {tokens.error instanceof Error && tokens.error.message
                ? tokens.error.message
                : t('admins.apiTokenLoadFailed', { defaultValue: 'Could not load your tokens' })}
            </p>
          ) : rows.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {t('admins.apiTokenNone', { defaultValue: 'No tokens yet.' })}
            </p>
          ) : (
            <div className="flex flex-col">
              {rows.map((row, index) => (
                <div key={row.id}>
                  {index > 0 && <Separator />}
                  <div className="flex items-center justify-between gap-4 py-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium">{row.name}</span>
                        <Badge variant={row.enabled ? 'default' : 'secondary'}>
                          {row.enabled
                            ? t('status.active', { defaultValue: 'Active' })
                            : t('status.disabled', { defaultValue: 'Disabled' })}
                        </Badge>
                      </div>
                      <p className="text-muted-foreground text-xs">
                        {t('admins.createdAt', { defaultValue: 'Created' })}: {formatDate(row.createdAt)}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={row.enabled}
                        onCheckedChange={(next) => toggleMutation.mutate({ id: row.id, enabled: next })}
                        aria-label={t('admins.apiTokenToggle', { defaultValue: 'Enable or disable' })}
                      />
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => deleteMutation.mutate(row.id)}
                        aria-label={t('delete', { defaultValue: 'Delete' })}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* create */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('admins.apiTokenCreate', { defaultValue: 'Create token' })}</DialogTitle>
            <DialogDescription>
              {t('admins.apiTokenNameHint', { defaultValue: 'Give it a name you will recognise later.' })}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="api-token-name">{t('name', { defaultValue: 'Name' })}</Label>
            <Input
              id="api-token-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="ci-deploy"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t('cancel')}
            </Button>
            <Button disabled={!name.trim() || createMutation.isPending} onClick={() => createMutation.mutate(name.trim())}>
              {t('create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* one-time reveal */}
      <Dialog open={plaintext !== null} onOpenChange={(open) => !open && setPlaintext(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('admins.apiTokenReady', { defaultValue: 'Your token' })}</DialogTitle>
            <DialogDescription>
              {t('admins.apiTokenOnce', {
                defaultValue: 'Copy it now — it cannot be shown again. Send it as an Authorization: Bearer header.',
              })}
            </DialogDescription>
          </DialogHeader>
          <div className="bg-muted flex items-center gap-2 rounded-md p-3">
            <code className="min-w-0 flex-1 overflow-x-auto text-xs">{plaintext}</code>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => {
                navigator.clipboard?.writeText(plaintext ?? '');
                toast.success(t('copied', { defaultValue: 'Copied' }));
              }}
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
