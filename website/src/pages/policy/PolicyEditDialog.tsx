import { useEffect, useState } from 'react'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  FormHelperText,
  InputAdornment,
  MenuItem,
  Radio,
  RadioGroup,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material'
import type { Policy, PolicyPayload } from '../../api/types'
import { policyApi } from '../../api/policy'
import { errorMessage } from '../../api/client'

export interface PolicyTemplate {
  key: string
  label: string
  description: string
  payload: Partial<PolicyPayload>
}

/** 内置策略模板：一键填充典型策略参数 */
export const POLICY_TEMPLATES: PolicyTemplate[] = [
  {
    key: 'ssh',
    label: 'SSH 爆破防护',
    description: '22 端口入站连接频率过高时临时封禁，多次触发升级封禁',
    payload: {
      name: 'SSH 爆破防护',
      direction: 'in',
      protocol: 'tcp',
      port: 22,
      conn_rate: 30,
      window_sec: 60,
      action: 'ban',
      ban_sec: 1800,
      risk_score: 1,
      cooldown_sec: 600,
      risk_ban_threshold: 3,
      risk_ban_sec: 86400,
    },
  },
  {
    key: 'scan',
    label: '端口扫描检测',
    description: '单个 IP 短时间内触碰大量端口时封禁',
    payload: {
      name: '端口扫描检测',
      direction: 'in',
      protocol: 'any',
      port: 0,
      distinct_ports: 50,
      window_sec: 300,
      action: 'ban',
      ban_sec: 3600,
      risk_score: 2,
      cooldown_sec: 600,
      risk_ban_threshold: 2,
      risk_ban_sec: 86400,
    },
  },
  {
    key: 'bandwidth',
    label: '单 IP 带宽限速',
    description: '每个源 IP 超过限速值后内核丢弃超限流量（hashlimit 令牌桶）',
    payload: {
      name: '单 IP 带宽限速',
      direction: 'in',
      protocol: 'any',
      port: 0,
      action: 'rate_limit',
      limit_kbps: 51200,
      burst_kbps: 102400,
      cooldown_sec: 300,
    },
  },
  {
    key: 'flood',
    label: '大流量攻击封禁',
    description: '入站速率超阈值时临时封禁，多次触发长期封禁',
    payload: {
      name: '大流量攻击封禁',
      direction: 'in',
      protocol: 'any',
      port: 0,
      rate_kbps: 51200,
      window_sec: 60,
      action: 'ban',
      ban_sec: 600,
      risk_score: 1,
      cooldown_sec: 300,
      risk_ban_threshold: 3,
      risk_ban_sec: 86400,
    },
  },
  {
    key: 'egress',
    label: '出站外传监测',
    description: '出站流量持续偏高时标记风险，累计多次自动封禁',
    payload: {
      name: '出站外传监测',
      direction: 'out',
      protocol: 'any',
      port: 0,
      rate_kbps: 20480,
      window_sec: 300,
      action: 'mark',
      risk_score: 1,
      cooldown_sec: 600,
      risk_ban_threshold: 5,
      risk_ban_sec: 3600,
    },
  },
]

interface PolicyEditDialogProps {
  open: boolean
  /** 编辑目标；null 表示新建 */
  policy: Policy | null
  onClose: () => void
  onMessage: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  onSaved: () => void
}

const emptyForm = (): PolicyPayload => ({
  name: '',
  enabled: true,
  direction: 'in',
  protocol: 'any',
  port: 0,
  rate_kbps: 0,
  total_mb: 0,
  conn_rate: 0,
  distinct_ports: 0,
  window_sec: 300,
  action: 'mark',
  limit_kbps: 0,
  burst_kbps: 0,
  ban_sec: 600,
  risk_score: 1,
  risk_ban_threshold: 0,
  risk_ban_sec: 0,
  cooldown_sec: 300,
})

/** 策略新建/编辑对话框：匹配 → 触发 → 动作 → 风险升级 */
export const PolicyEditDialog = ({ open, policy, onClose, onMessage, onSaved }: PolicyEditDialogProps) => {
  const [form, setForm] = useState<PolicyPayload>(emptyForm())
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    if (policy) {
      const { id: _id, created_at: _c, updated_at: _u, ...rest } = policy
      setForm({ ...emptyForm(), ...rest })
    } else {
      setForm(emptyForm())
    }
  }, [open, policy])

  const set = <K extends keyof PolicyPayload>(key: K, value: PolicyPayload[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const applyTemplate = (template: PolicyTemplate) => {
    setForm({ ...emptyForm(), ...template.payload } as PolicyPayload)
  }

  const numField = (key: keyof PolicyPayload, label: string, end?: string, placeholder?: string) => {
    const value = form[key] as number
    return (
      <TextField
        label={label}
        type="number"
        size="small"
        fullWidth
        value={value || ''}
        placeholder={placeholder ?? '0 表示不启用'}
        onChange={(event) => set(key, (parseInt(event.target.value, 10) || 0) as never)}
        InputProps={end ? { endAdornment: <InputAdornment position="end">{end}</InputAdornment> } : undefined}
        inputProps={{ min: 0 }}
      />
    )
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      if (policy) {
        await policyApi.update(policy.id, form)
        onMessage(`策略「${form.name}」已更新`)
      } else {
        await policyApi.create(form)
        onMessage(`策略「${form.name}」已创建`)
      }
      onSaved()
      onClose()
    } catch (err) {
      onMessage(errorMessage(err, '保存策略失败'), 'error')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{policy ? `编辑策略「${policy.name}」` : '新建策略'}</DialogTitle>
      <DialogContent dividers>
        {!policy && (
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mb: 2 }}>
            {POLICY_TEMPLATES.map((template) => (
              <Button key={template.key} size="small" variant="outlined" onClick={() => applyTemplate(template)}>
                {template.label}
              </Button>
            ))}
          </Stack>
        )}

        <Stack spacing={2}>
          <Stack direction="row" spacing={2} alignItems="center">
            <TextField
              label="策略名称"
              size="small"
              required
              fullWidth
              value={form.name}
              onChange={(event) => set('name', event.target.value)}
            />
            <FormControlLabel
              control={<Switch checked={form.enabled} onChange={(event) => set('enabled', event.target.checked)} />}
              label="启用"
            />
          </Stack>

          <Divider>
            <Typography variant="caption" color="text.secondary">
              匹配条件
            </Typography>
          </Divider>
          <Stack direction="row" spacing={2}>
            <TextField
              select
              label="方向"
              size="small"
              fullWidth
              value={form.direction}
              onChange={(event) => set('direction', event.target.value as PolicyPayload['direction'])}
            >
              <MenuItem value="in">入站 (in)</MenuItem>
              <MenuItem value="out">出站 (out)</MenuItem>
              <MenuItem value="both">双向 (both)</MenuItem>
            </TextField>
            <TextField
              select
              label="协议"
              size="small"
              fullWidth
              value={form.protocol}
              onChange={(event) => set('protocol', event.target.value as PolicyPayload['protocol'])}
            >
              <MenuItem value="any">任意</MenuItem>
              <MenuItem value="tcp">TCP</MenuItem>
              <MenuItem value="udp">UDP</MenuItem>
            </TextField>
            <TextField
              label="端口"
              type="number"
              size="small"
              fullWidth
              value={form.port || ''}
              placeholder="0 = 任意"
              onChange={(event) => set('port', parseInt(event.target.value, 10) || 0)}
              inputProps={{ min: 0, max: 65535 }}
            />
          </Stack>

          {form.action !== 'rate_limit' && (
            <>
              <Divider>
                <Typography variant="caption" color="text.secondary">
                  触发条件（任一超限即触发，0 = 不启用）
                </Typography>
              </Divider>
              <Stack direction="row" spacing={2}>
                {numField('rate_kbps', '速率阈值', 'KB/s')}
                {numField('total_mb', '累计流量', 'MB')}
              </Stack>
              <Stack direction="row" spacing={2}>
                {numField('conn_rate', '新建连接数')}
                {numField('distinct_ports', '触碰端口数', undefined, '扫描检测')}
              </Stack>
              {numField('window_sec', '统计窗口', '秒')}
            </>
          )}

          <Divider>
            <Typography variant="caption" color="text.secondary">
              动作
            </Typography>
          </Divider>
          <RadioGroup
            row
            value={form.action}
            onChange={(event) => set('action', event.target.value as PolicyPayload['action'])}
          >
            <FormControlLabel value="rate_limit" control={<Radio size="small" />} label="限速" />
            <FormControlLabel value="ban" control={<Radio size="small" />} label="临时封禁" />
            <FormControlLabel value="mark" control={<Radio size="small" />} label="仅标记风险" />
          </RadioGroup>
          {form.action === 'rate_limit' && (
            <Stack direction="row" spacing={2}>
              {numField('limit_kbps', '限速值', 'KB/s')}
              {numField('burst_kbps', '突发容量', 'KB', '默认 2 倍限速值')}
            </Stack>
          )}
          {form.action === 'ban' && numField('ban_sec', '禁用时长', '秒')}

          <Divider>
            <Typography variant="caption" color="text.secondary">
              风险与升级
            </Typography>
          </Divider>
          <Stack direction="row" spacing={2}>
            {numField('risk_score', '每次触发风险分', undefined, '默认 1')}
            {numField('cooldown_sec', '触发冷却', '秒')}
          </Stack>
          <Stack direction="row" spacing={2}>
            {numField('risk_ban_threshold', '升级阈值（风险分累计）')}
            {numField('risk_ban_sec', '升级封禁时长', '秒')}
          </Stack>
          <FormHelperText>
            风险分在统计窗口（默认 24 小时）内累计，达到升级阈值后自动临时封禁并清零重计
          </FormHelperText>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button variant="contained" onClick={() => void handleSave()} disabled={saving || !form.name.trim()}>
          保存
        </Button>
      </DialogActions>
    </Dialog>
  )
}
