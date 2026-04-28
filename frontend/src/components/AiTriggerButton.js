import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
/**
 * Phase 15-C: AI Trigger button for a node.
 * Sends a trigger request and polls job status until done/failed (60s timeout).
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
const STATUS_LABEL = {
    pending: '等待中',
    processing: '处理中',
    done: '完成',
    failed: '失败',
};
const STATUS_COLOR = {
    pending: '#f5a623',
    processing: '#4a8eff',
    done: '#52c41a',
    failed: '#f5222d',
};
const POLL_INTERVAL_MS = 2000;
const POLL_MAX = 30; // 60 seconds total
export default function AiTriggerButton({ lakeId, nodeId, promptTemplateId, inputNodeIds, onDone, onFail }) {
    const [job, setJob] = useState(null);
    const [error, setError] = useState(null);
    const [triggering, setTriggering] = useState(false);
    const pollCount = useRef(0);
    const pollTimer = useRef(null);
    const stopPolling = useCallback(() => {
        if (pollTimer.current !== null) {
            clearTimeout(pollTimer.current);
            pollTimer.current = null;
        }
    }, []);
    const poll = useCallback(async (currentJob) => {
        if (pollCount.current >= POLL_MAX) {
            stopPolling();
            setJob(prev => prev ? { ...prev, status: 'failed', error: '轮询超时（60s）' } : prev);
            return;
        }
        try {
            const updated = await api.aiStatus(lakeId, nodeId);
            setJob(updated);
            if (updated.status === 'done') {
                stopPolling();
                onDone?.(updated);
                return;
            }
            if (updated.status === 'failed') {
                stopPolling();
                onFail?.(updated);
                return;
            }
            pollCount.current += 1;
            pollTimer.current = setTimeout(() => poll(updated), POLL_INTERVAL_MS);
        }
        catch {
            // transient network error — keep polling
            pollCount.current += 1;
            pollTimer.current = setTimeout(() => poll(currentJob), POLL_INTERVAL_MS);
        }
    }, [lakeId, nodeId, onDone, onFail, stopPolling]);
    useEffect(() => () => stopPolling(), [stopPolling]);
    const handleTrigger = useCallback(async () => {
        if (triggering || (job && (job.status === 'pending' || job.status === 'processing')))
            return;
        setError(null);
        setTriggering(true);
        stopPolling();
        pollCount.current = 0;
        try {
            const newJob = await api.aiTrigger(lakeId, nodeId, {
                prompt_template_id: promptTemplateId,
                input_node_ids: inputNodeIds,
            });
            setJob(newJob);
            if (newJob.status !== 'done' && newJob.status !== 'failed') {
                pollTimer.current = setTimeout(() => poll(newJob), POLL_INTERVAL_MS);
            }
            else if (newJob.status === 'done') {
                onDone?.(newJob);
            }
            else {
                onFail?.(newJob);
            }
        }
        catch (e) {
            setError(String(e?.message || e));
        }
        finally {
            setTriggering(false);
        }
    }, [triggering, job, lakeId, nodeId, promptTemplateId, inputNodeIds, poll, stopPolling, onDone, onFail]);
    const isRunning = job?.status === 'pending' || job?.status === 'processing';
    const disabled = triggering || isRunning;
    return (_jsxs("div", { style: { display: 'inline-flex', flexDirection: 'column', gap: 6, alignItems: 'flex-start' }, children: [_jsxs("button", { onClick: handleTrigger, disabled: disabled, title: "\u89E6\u53D1 AI \u5904\u7406\u5F53\u524D\u8282\u70B9", style: {
                    padding: '6px 14px',
                    borderRadius: 6,
                    border: 'none',
                    cursor: disabled ? 'not-allowed' : 'pointer',
                    background: disabled ? '#2a2a3a' : '#4a8eff',
                    color: '#fff',
                    fontWeight: 600,
                    fontSize: 13,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    opacity: disabled ? 0.7 : 1,
                    transition: 'opacity 0.2s, background 0.2s',
                }, children: [_jsx("span", { style: { fontSize: 15 }, children: "\u2726" }), isRunning ? 'AI 处理中…' : 'AI 触发'] }), job && (_jsxs("div", { style: {
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    fontSize: 12,
                    color: '#aaa',
                }, children: [_jsx("span", { style: {
                            width: 8,
                            height: 8,
                            borderRadius: '50%',
                            background: STATUS_COLOR[job.status],
                            display: 'inline-block',
                            flexShrink: 0,
                        } }), _jsx("span", { style: { color: STATUS_COLOR[job.status] }, children: STATUS_LABEL[job.status] }), isRunning && job.progress_pct > 0 && (_jsxs("span", { style: { color: '#888' }, children: [job.progress_pct, "%"] })), job.status === 'failed' && job.error && (_jsxs("span", { style: { color: '#f5222d', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }, children: ["\u2014 ", job.error] })), isRunning && (_jsxs("span", { style: { color: '#555', fontSize: 11 }, children: ["\u8F6E\u8BE2 ", pollCount.current, "/", POLL_MAX] })), !isRunning && (_jsx("button", { onClick: handleTrigger, style: {
                            background: 'none',
                            border: 'none',
                            color: '#4a8eff',
                            cursor: 'pointer',
                            fontSize: 11,
                            padding: '0 2px',
                        }, children: "\u91CD\u8BD5" }))] })), error && (_jsx("div", { style: { color: '#f5222d', fontSize: 12 }, children: error }))] }));
}
