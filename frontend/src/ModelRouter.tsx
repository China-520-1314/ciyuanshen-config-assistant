import { useEffect, useState } from 'react';
import { Activity, ArrowRight, Play, Square, RefreshCw } from 'lucide-react';
import { GetRouterModels, GetModelRouterStatus, StartModelRouter, StopModelRouter, GetRouterAccountOptions } from '../wailsjs/go/main/App';

type Status = { running: boolean; model: string; alias: string; address: string; requests: number; failures: number; lastError: string; backupPath: string };
type Group = { name: string; description: string; ratio: string; models: { id: string }[] };
type Key = { provisionId: string; group: string; groupDescription?: string; name?: string; models: { id: string }[]; existing?: boolean };
type AccountOptions = { groups: Group[]; existingKeys?: Key[] };
type Request = { apiKey: string; provisionId: string; model: string; useExistingKey: boolean };
type RouterBridge = {
  GetRouterModels(request: Request): Promise<{ models: { id: string }[] }>;
  GetModelRouterStatus(): Promise<Status>;
  StartModelRouter(request: Request): Promise<Status>;
  StopModelRouter(): Promise<Status>;
  GetRouterAccountOptions(): Promise<AccountOptions>;
};
const bridge = (): RouterBridge | undefined => (window as unknown as { go?: { main?: { App?: unknown } } }).go?.main?.App
  ? { GetRouterModels, GetModelRouterStatus, StartModelRouter, StopModelRouter, GetRouterAccountOptions } : undefined;
const initial: Status = { running: false, model: '', alias: 'gpt-5.6-terra', address: '', requests: 0, failures: 0, lastError: '', backupPath: '' };

export default function ModelRouter() {
  const [status, setStatus] = useState(initial);
  const [existing, setExisting] = useState(true);
  const [key, setKey] = useState('');
  const [models, setModels] = useState<string[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [keys, setKeys] = useState<Key[]>([]);
  const [selectedGroup, setSelectedGroup] = useState('');
  const [provisionId, setProvisionId] = useState('');
  const [model, setModel] = useState('');
  const [filter, setFilter] = useState('');
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    const refresh = () => { void bridge()?.GetModelRouterStatus().then(value => { if (active) setStatus(value); }).catch(() => { if (active) setError('读取路由状态失败，请重新打开此页面'); }); };
    refresh(); const timer = window.setInterval(refresh, 2500);
    return () => { active = false; window.clearInterval(timer); };
  }, []);
  function resetModels() { setModels([]); setModel(''); setFilter(''); setError(''); setNotice(''); }
  async function run(action: 'models' | 'start' | 'stop') {
    const api = bridge(); if (!api) { setError('请在桌面安装版中使用本地路由'); return; }
    setBusy(action); setError(''); setNotice('');
    const request = { apiKey: key, provisionId, model, useExistingKey: existing };
    try {
      if (action === 'models') {
        setModels([]); setModel('');
        if (existing) {
          const account = await api.GetRouterAccountOptions();
          setGroups(account.groups || []); setKeys(account.existingKeys || []);
          const available = [...(account.existingKeys || []).flatMap(k => k.models), ...(account.groups || []).flatMap(g => g.models)].map(m => m.id);
          const ids = [...new Set(available)].sort(); setModels(ids); setModel('');
          if ((account.existingKeys || []).length) { setProvisionId(account.existingKeys![0].provisionId); setSelectedGroup(account.existingKeys![0].group); }
          setNotice(ids.length ? `已读取 ${ids.length} 个模型。已自动选择第一个可用分组。` : '账号没有可用模型');
          return;
        }
        const result = await api.GetRouterModels(request);
        const ids = result.models.map(item => item.id).sort(); setModels(ids); setModel('');
        setNotice(ids.length ? `已读取 ${ids.length} 个模型。请选择支持对话和工具调用的模型。` : '此 Key 没有可用模型');
      } else if (action === 'start') {
        setStatus(await api.StartModelRouter(request)); setKey('');
        setNotice('路由已启动。请重新打开 Codex 并新建对话，发送一句话测试连接。使用期间保持助手运行。');
      } else {
        setStatus(await api.StopModelRouter()); resetModels();
        setNotice('已停止路由并恢复原配置。请重新打开 Codex。');
      }
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(''); }
  }
  const locked = Boolean(busy) || status.running;
  const visible = models.filter(id => id.toLowerCase().includes(filter.toLowerCase()));
  const providers = [...new Set(visible.map(id => id.split(/[-/:]/)[0].toLowerCase()))].sort();
  return <div className="content-stack narrow-stack router-page">
    <div className="page-intro"><div><p className="eyebrow">模型路由</p><h2>在 Codex 里使用更多模型</h2><p>选择词元神 Key 和实际模型，一键连接 Claude、Gemini、Grok、DeepSeek 等。</p></div></div>
    <section className="router-card">
      <h3>1 · 选择用于连接的 Key</h3>
      <div className="router-choices">
        <label><input type="radio" name="router-key-source" checked={existing} disabled={locked} onChange={() => { setExisting(true); setKey(''); resetModels(); }} />使用 Codex 当前的词元神 Key</label>
        <label><input type="radio" name="router-key-source" checked={!existing} disabled={locked} onChange={() => { setExisting(false); resetModels(); }} />输入另一个词元神 Key</label>
      </div>
      {!existing && <div className="field-block"><label htmlFor="router-key">词元神 API Key</label><input id="router-key" type="password" autoComplete="off" value={key} disabled={locked} placeholder="粘贴支持目标模型分组的 Key" onChange={e => { setKey(e.target.value); resetModels(); }} /></div>}
      {existing && keys.length > 0 && <div className="field-block"><label htmlFor="router-group">模型对应分组</label><select id="router-group" value={provisionId} onChange={e => { const k = keys.find(x => x.provisionId === e.target.value); setProvisionId(e.target.value); setSelectedGroup(k?.group || ''); setModel(''); }} disabled={locked}>{keys.map(k => <option key={k.provisionId} value={k.provisionId}>{k.group || '未命名分组'} · {k.name || '已创建 Key'}</option>)}</select>{keys.find(k => k.provisionId === provisionId) && <p className="field-note">倍率：{groups.find(g => g.name === selectedGroup)?.ratio || '—'} · {keys.find(k => k.provisionId === provisionId)?.groupDescription || groups.find(g => g.name === selectedGroup)?.description || '暂无分组描述'}</p>}</div>}
      {existing && groups.length > 1 && keys.length === 0 && <div className="field-block"><label htmlFor="router-group">选择模型分组</label><select id="router-group" value={selectedGroup} onChange={e => { const g = groups.find(x => x.name === e.target.value); setSelectedGroup(e.target.value); setModels(g?.models.map(m => m.id) || []); setModel(''); }} disabled={locked}>{groups.map(g => <option key={g.name} value={g.name}>{g.name} · {g.ratio}x</option>)}</select><p className="field-note">{groups.find(g => g.name === selectedGroup)?.description || '暂无分组描述'}</p></div>}
      <button className="secondary-button" disabled={locked || (!existing && !key.trim())} onClick={() => void run('models')}><RefreshCw size={15} />{busy === 'models' ? '读取中…' : '读取可用模型与分组'}</button>
      <p className="field-note">当前 Key 若属于其他服务商，请选择输入词元神 Key。Key 只用于本次路由，不保存到助手。</p>
    </section>
    <section className="router-card">
      <h3>2 · 选择真正回答你的模型</h3>
      <div className="field-block"><label htmlFor="router-filter">搜索模型</label><input id="router-filter" disabled={locked || !models.length} value={filter} onChange={e => setFilter(e.target.value)} placeholder="输入 claude、gemini、grok 或 deepseek" /></div>
      <div className="field-block"><label htmlFor="router-model">实际使用模型</label><select id="router-model" value={model} disabled={locked || !models.length} onChange={e => { const id=e.target.value; setModel(id); const match=keys.find(k => k.models.some(m => m.id === id)); if (match) { setProvisionId(match.provisionId); setSelectedGroup(match.group); } }}><option value="">请选择模型</option>{model && !visible.includes(model) && <option value={model}>{model}</option>}{providers.map(provider => <optgroup key={provider} label={provider.toUpperCase()}>{visible.filter(id => id.split(/[-/:]/)[0].toLowerCase() === provider).map(id => <option key={id} value={id}>{id}</option>)}</optgroup>)}</select></div>
      <div className="router-mapping"><span>Codex 显示<br /><strong>{status.alias}</strong></span><ArrowRight size={22} /><span>实际请求<br /><strong>{status.running ? status.model : model || '等待选择'}</strong></span></div>
      <p className="field-note">模型列表汇总账号所有可用分组，并按供应商分类；当前选择的分组 Key 必须实际拥有该模型权限。左侧是供 Codex 使用的模型别名，右侧才是词元神实际收到的模型名。</p>
    </section>
    <section className="router-card">
      <h3>3 · 一键启动，回到 Codex 开始使用</h3>
      <p>启动时自动备份并切换 Codex 配置；停止或正常退出助手时恢复。如果配置被其他程序修改，会保留修改和恢复记录。重新打开 Codex、新建对话后生效。</p>
      <div className="router-actions"><button className="primary-button" disabled={locked || !model || (!existing && !key.trim())} onClick={() => void run('start')}><Play size={16} />{busy === 'start' ? '启动中…' : '一键启动并配置 Codex'}</button><button className="secondary-button" disabled={Boolean(busy) || (!status.running && !status.lastError)} onClick={() => void run('stop')}><Square size={15} />{busy === 'stop' ? '恢复中…' : '停止并恢复配置'}</button></div>
      <div className="router-live" role="status"><Activity size={17} /><strong>{status.running ? '运行中' : '未启动'}</strong>{status.running && <span>请求 {status.requests} 次 · 失败 {status.failures} 次</span>}</div>
      {status.running && <p className="field-note">本机地址：{status.address} · 使用期间请保持助手运行。</p>}
      {(error || status.lastError) && <div className="router-error" role="alert">{error || status.lastError}</div>}
      {notice && <p className="router-notice" role="status">{notice}</p>}
      <p className="field-note">支持文本流式回复、图片输入、函数与补丁工具。目标模型须支持相应能力。暂不支持内置联网搜索、服务端对话续接及远端压缩；遇到不支持的输入会明确报错。长对话请新建会话。</p>
    </section>
  </div>;
}
