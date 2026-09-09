import { useEffect, useState } from 'react'
import { Box, MenuItem, Select, TextField, Typography } from '@mui/material'

interface RowsPerPageControlProps {
  value: number
  onChange: (value: number) => void
  options?: number[]
}

const MAX_ROWS = 10000

/** 「每页显示条数」控件：下拉选择 + 自定义输入 */
export const RowsPerPageControl = ({ value, onChange, options = [10, 25, 50, 100] }: RowsPerPageControlProps) => {
  const [custom, setCustom] = useState(String(value))

  // 外部分页大小变化时同步输入框
  useEffect(() => {
    setCustom(String(value))
  }, [value])

  const commitCustom = () => {
    const parsed = parseInt(custom, 10)
    if (Number.isFinite(parsed) && parsed > 0 && parsed <= MAX_ROWS) {
      onChange(parsed)
    } else {
      setCustom(String(value))
    }
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
      <Typography variant="body2" color="text.secondary">
        每页显示：
      </Typography>
      <Select
        size="small"
        value={options.includes(value) ? value : ''}
        onChange={(event) => {
          const next = Number(event.target.value)
          if (next > 0) {
            onChange(next)
          }
        }}
        displayEmpty
        sx={{ minWidth: 100 }}
      >
        {options.map((option) => (
          <MenuItem key={option} value={option}>
            {option} 条
          </MenuItem>
        ))}
      </Select>
      <TextField
        size="small"
        type="number"
        label="自定义数量"
        value={custom}
        onChange={(event) => setCustom(event.target.value)}
        onBlur={commitCustom}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.currentTarget.blur()
          }
        }}
        inputProps={{ min: 1, max: MAX_ROWS }}
        sx={{ width: 120 }}
      />
    </Box>
  )
}
