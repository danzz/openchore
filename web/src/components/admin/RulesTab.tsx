import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { Award, Flame, Trash2, Plus, ShieldAlert, Sparkles } from 'lucide-react';
import { api } from '../../api';
import type { StreakRewardItem } from '../../types';
import { ExportConfigSection } from './ExportConfigSection';
import { FormMessage, type Msg } from './FormMessage';
import ConfirmModal from '../ConfirmModal/ConfirmModal';
import ui from './ui.module.css';

export const RulesTab: React.FC = () => {
  const { t } = useTranslation();

  // 1. Missed Chores state
  const [cutoffHour, setCutoffHour] = useState('21');
  const [penaltyMode, setPenaltyMode] = useState<'full' | 'flat'>('full');
  const [flatPenalty, setFlatPenalty] = useState('5');
  const [missedSaving, setMissedSaving] = useState(false);
  const [missedMsg, setMissedMsg] = useState<Msg>(null);

  // 2. Points Decay state
  const [decayEnabled, setDecayEnabled] = useState(false);
  const [decayPercent, setDecayPercent] = useState('10');
  const [decayHour, setDecayHour] = useState('22');
  const [decaySaving, setDecaySaving] = useState(false);
  const [decayMsg, setDecayMsg] = useState<Msg>(null);

  // 3. Redemption Control state
  const [allowRedemptions, setAllowRedemptions] = useState(true);
  const [redemptionSaving, setRedemptionSaving] = useState(false);
  const [redemptionMsg, setRedemptionMsg] = useState<Msg>(null);

  // 4. Streak Rewards state
  const [streaks, setStreaks] = useState<StreakRewardItem[]>([]);
  const [streaksLoading, setStreaksLoading] = useState(false);
  const [showAddStreak, setShowAddStreak] = useState(false);
  const [newStreakDays, setNewStreakDays] = useState('3');
  const [newStreakBonus, setNewStreakBonus] = useState('10');
  const [newStreakLabel, setNewStreakLabel] = useState('');
  const [addingStreak, setAddingStreak] = useState(false);
  const [streakMsg, setStreakMsg] = useState<Msg>(null);

  // Load initial settings
  useEffect(() => {
    // Missed chores
    api.admin.getSetting('missed_chores_cutoff_hour')
      .then(d => { if (d.value) setCutoffHour(d.value); })
      .catch(() => {});
    api.admin.getSetting('missed_chores_penalty_mode')
      .then(d => { if (d.value === 'flat' || d.value === 'full') setPenaltyMode(d.value); })
      .catch(() => {});
    api.admin.getSetting('missed_chores_flat_penalty')
      .then(d => { if (d.value) setFlatPenalty(d.value); })
      .catch(() => {});

    // Decay settings
    api.admin.getSetting('points_decay_enabled')
      .then(d => { setDecayEnabled(d.value === 'true'); })
      .catch(() => {});
    api.admin.getSetting('points_decay_daily_percent')
      .then(d => { if (d.value) setDecayPercent(d.value); })
      .catch(() => {});
    api.admin.getSetting('points_decay_cutoff_hour')
      .then(d => { if (d.value) setDecayHour(d.value); })
      .catch(() => {});

    // Redemption
    api.admin.getSetting('allow_redemptions')
      .then(d => { if (d.value !== '') setAllowRedemptions(d.value !== 'false'); })
      .catch(() => {});
  }, []);

  const loadStreaks = useCallback(async () => {
    setStreaksLoading(true);
    try {
      const list = await api.streaks.listRewards();
      setStreaks(list.sort((a, b) => a.streak_days - b.streak_days));
    } catch (e) {
      console.error(e);
    }
    setStreaksLoading(false);
  }, []);

  useEffect(() => {
    loadStreaks();
  }, [loadStreaks]);

  // Handlers
  const handleSaveMissed = async (e: React.FormEvent) => {
    e.preventDefault();
    setMissedSaving(true);
    setMissedMsg(null);
    try {
      await api.admin.setSetting('missed_chores_cutoff_hour', cutoffHour);
      await api.admin.setSetting('missed_chores_penalty_mode', penaltyMode);
      await api.admin.setSetting('missed_chores_flat_penalty', flatPenalty);
      setMissedMsg({ type: 'success', text: t('admin.rulesTab.saveSuccess') });
    } catch {
      setMissedMsg({ type: 'error', text: t('admin.rulesTab.saveError') });
    }
    setMissedSaving(false);
  };

  const handleSaveDecay = async (e: React.FormEvent) => {
    e.preventDefault();
    setDecaySaving(true);
    setDecayMsg(null);
    try {
      await api.admin.setSetting('points_decay_enabled', decayEnabled ? 'true' : 'false');
      await api.admin.setSetting('points_decay_daily_percent', decayPercent);
      await api.admin.setSetting('points_decay_cutoff_hour', decayHour);
      setDecayMsg({ type: 'success', text: t('admin.rulesTab.saveSuccess') });
    } catch {
      setDecayMsg({ type: 'error', text: t('admin.rulesTab.saveError') });
    }
    setDecaySaving(false);
  };

  const handleSaveRedemption = async (e: React.FormEvent) => {
    e.preventDefault();
    setRedemptionSaving(true);
    setRedemptionMsg(null);
    try {
      await api.admin.setSetting('allow_redemptions', allowRedemptions ? 'true' : 'false');
      setRedemptionMsg({ type: 'success', text: t('admin.rulesTab.saveSuccess') });
    } catch {
      setRedemptionMsg({ type: 'error', text: t('admin.rulesTab.saveError') });
    }
    setRedemptionSaving(false);
  };

  const handleAddStreak = async (e: React.FormEvent) => {
    e.preventDefault();
    const days = parseInt(newStreakDays, 10);
    const bonus = parseInt(newStreakBonus, 10);
    if (!days || days <= 0 || !bonus || bonus <= 0 || !newStreakLabel.trim()) return;

    setAddingStreak(true);
    setStreakMsg(null);
    try {
      await api.streaks.createReward({
        streak_days: days,
        bonus_points: bonus,
        label: newStreakLabel.trim(),
      });
      setNewStreakLabel('');
      setShowAddStreak(false);
      await loadStreaks();
      setStreakMsg({ type: 'success', text: t('admin.rulesTab.saveSuccess') });
    } catch {
      setStreakMsg({ type: 'error', text: t('admin.rulesTab.saveError') });
    }
    setAddingStreak(false);
  };

  const [streakToDelete, setStreakToDelete] = useState<StreakRewardItem | null>(null);

  const handleConfirmDeleteStreak = async () => {
    if (!streakToDelete) return;
    try {
      await api.streaks.deleteReward(streakToDelete.id);
      setStreakToDelete(null);
      await loadStreaks();
    } catch (e) {
      console.error(e);
      throw e;
    }
  };

  return (
    <div className={ui.page}>
      <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.rulesTab.title')}</h2>
          <p className={ui.pageSub}>{t('admin.rulesTab.subtitle')}</p>
        </div>
      </div>

      {/* 1. Missed Chores Rules Card */}
      <section className={clsx(ui.card, ui.section)}>
        <div className={ui.sectionHead}>
          <h3 className={ui.sectionTitle}>
            <ShieldAlert size={20} aria-hidden />
            {t('admin.rulesTab.missedTitle')}
          </h3>
        </div>
        <p className={ui.sectionDesc}>{t('admin.rulesTab.missedDesc')}</p>
        <form onSubmit={handleSaveMissed} className={ui.form}>
          <div className={ui.field}>
            <label className={ui.label} htmlFor="cutoffHourInput">
              {t('admin.rulesTab.cutoffHour')}
            </label>
            <input
              id="cutoffHourInput"
              type="number"
              min="0"
              max="23"
              className={ui.input}
              value={cutoffHour}
              onChange={(e) => setCutoffHour(e.target.value)}
              required
            />
            <span className={ui.help}>{t('admin.rulesTab.cutoffHourHint')}</span>
          </div>

          <div className={ui.field}>
            <label className={ui.label} htmlFor="penaltyModeSelect">
              {t('admin.rulesTab.penaltyMode')}
            </label>
            <select
              id="penaltyModeSelect"
              className={ui.select}
              value={penaltyMode}
              onChange={(e) => setPenaltyMode(e.target.value as 'full' | 'flat')}
            >
              <option value="full">{t('admin.rulesTab.modeFull')}</option>
              <option value="flat">{t('admin.rulesTab.modeFlat')}</option>
            </select>
          </div>

          {penaltyMode === 'flat' && (
            <div className={ui.field}>
              <label className={ui.label} htmlFor="flatPenaltyInput">
                {t('admin.rulesTab.flatPenalty')}
              </label>
              <input
                id="flatPenaltyInput"
                type="number"
                min="0"
                className={ui.input}
                value={flatPenalty}
                onChange={(e) => setFlatPenalty(e.target.value)}
                required
              />
            </div>
          )}

          <FormMessage msg={missedMsg} />
          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={missedSaving}>
              {missedSaving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        </form>
      </section>

      {/* 2. Points Decay Card */}
      <section className={clsx(ui.card, ui.section)}>
        <div className={ui.sectionHead}>
          <h3 className={ui.sectionTitle}>
            <Flame size={20} aria-hidden />
            {t('admin.rulesTab.decayTitle')}
          </h3>
        </div>
        <p className={ui.sectionDesc}>{t('admin.rulesTab.decayDesc')}</p>
        <form onSubmit={handleSaveDecay} className={ui.form}>
          <div className={ui.field}>
            <label className={ui.checkboxLabel}>
              <input
                type="checkbox"
                checked={decayEnabled}
                onChange={(e) => setDecayEnabled(e.target.checked)}
              />
              <span>{t('admin.rulesTab.decayEnabled')}</span>
            </label>
          </div>

          {decayEnabled && (
            <>
              <div className={ui.field}>
                <label className={ui.label} htmlFor="decayPercentInput">
                  {t('admin.rulesTab.decayPercent')}
                </label>
                <input
                  id="decayPercentInput"
                  type="number"
                  min="1"
                  max="100"
                  className={ui.input}
                  value={decayPercent}
                  onChange={(e) => setDecayPercent(e.target.value)}
                  required
                />
              </div>
              <div className={ui.field}>
                <label className={ui.label} htmlFor="decayHourInput">
                  {t('admin.rulesTab.decayHour')}
                </label>
                <input
                  id="decayHourInput"
                  type="number"
                  min="0"
                  max="23"
                  className={ui.input}
                  value={decayHour}
                  onChange={(e) => setDecayHour(e.target.value)}
                  required
                />
              </div>
            </>
          )}

          <FormMessage msg={decayMsg} />
          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={decaySaving}>
              {decaySaving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        </form>
      </section>

      {/* 3. Global Redemption Control */}
      <section className={clsx(ui.card, ui.section)}>
        <div className={ui.sectionHead}>
          <h3 className={ui.sectionTitle}>
            <Award size={20} aria-hidden />
            {t('admin.rulesTab.redemptionTitle')}
          </h3>
        </div>
        <p className={ui.sectionDesc}>{t('admin.rulesTab.redemptionDesc')}</p>
        <form onSubmit={handleSaveRedemption} className={ui.form}>
          <div className={ui.field}>
            <label className={ui.checkboxLabel}>
              <input
                type="checkbox"
                checked={allowRedemptions}
                onChange={(e) => setAllowRedemptions(e.target.checked)}
              />
              <span>{t('admin.rulesTab.allowRedemptions')}</span>
            </label>
          </div>

          <FormMessage msg={redemptionMsg} />
          <div className={ui.actionsEnd}>
            <button type="submit" className={ui.btnPrimary} disabled={redemptionSaving}>
              {redemptionSaving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        </form>
      </section>

      {/* 4. Streak Milestone Rewards Management */}
      <section className={clsx(ui.card, ui.section)}>
        <div className={ui.sectionHead}>
          <h3 className={ui.sectionTitle}>
            <Sparkles size={20} aria-hidden />
            {t('admin.rulesTab.streakTitle')}
          </h3>
          <button
            type="button"
            className={ui.btnSecondary}
            onClick={() => setShowAddStreak(!showAddStreak)}
          >
            <Plus size={16} aria-hidden />
            <span>{t('admin.rulesTab.addStreak')}</span>
          </button>
        </div>
        <p className={ui.sectionDesc}>{t('admin.rulesTab.streakDesc')}</p>

        {showAddStreak && (
          <form onSubmit={handleAddStreak} className={clsx(ui.form, ui.nestedForm)}>
            <div className={ui.row}>
              <div className={ui.field}>
                <label className={ui.label}>{t('admin.rulesTab.streakDays')}</label>
                <input
                  type="number"
                  min="1"
                  className={ui.input}
                  value={newStreakDays}
                  onChange={(e) => setNewStreakDays(e.target.value)}
                  required
                />
              </div>
              <div className={ui.field}>
                <label className={ui.label}>{t('admin.rulesTab.bonusPoints')}</label>
                <input
                  type="number"
                  min="1"
                  className={ui.input}
                  value={newStreakBonus}
                  onChange={(e) => setNewStreakBonus(e.target.value)}
                  required
                />
              </div>
            </div>
            <div className={ui.field}>
              <label className={ui.label}>{t('admin.rulesTab.milestoneLabel')}</label>
              <input
                type="text"
                className={ui.input}
                value={newStreakLabel}
                placeholder={t('admin.rulesTab.milestonePlaceholder')}
                onChange={(e) => setNewStreakLabel(e.target.value)}
                required
              />
            </div>
            <div className={ui.actionsEnd}>
              <button
                type="button"
                className={ui.btnText}
                onClick={() => setShowAddStreak(false)}
              >
                {t('common.cancel')}
              </button>
              <button type="submit" className={ui.btnPrimary} disabled={addingStreak}>
                {addingStreak ? t('common.saving') : t('common.save')}
              </button>
            </div>
          </form>
        )}

        <FormMessage msg={streakMsg} />

        {streaksLoading ? (
          <p className={ui.emptyNotice}>{t('common.loading')}</p>
        ) : streaks.length === 0 ? (
          <p className={ui.emptyNotice}>{t('admin.rulesTab.noStreaks')}</p>
        ) : (
          <div className={ui.hairlineList}>
            {streaks.map((item) => (
              <div key={item.id} className={ui.hairlineItem}>
                <div className={ui.hairlineMain}>
                  <div className={ui.hairlineTitle}>
                    <span className={ui.badge}>{item.streak_days} 天</span>
                    <strong>{item.label}</strong>
                  </div>
                  <div className={ui.hairlineSub}>
                    +{item.bonus_points} 奖励积分
                  </div>
                </div>
                <div className={ui.hairlineActions}>
                  <button
                    type="button"
                    className={ui.iconBtnDanger}
                    onClick={() => setStreakToDelete(item)}
                    title={t('common.delete')}
                    aria-label={t('common.delete')}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* 5. Full Config YAML Export & Backup */}
      <ExportConfigSection />

      {streakToDelete && (
        <ConfirmModal
          isOpen={!!streakToDelete}
          title={t('common.delete', { defaultValue: '删除连胜阶梯奖励' })}
          message={t('admin.rulesTab.deleteConfirm', { days: streakToDelete.streak_days })}
          confirmLabel={t('common.delete', { defaultValue: '确认删除' })}
          cancelLabel={t('common.cancel', { defaultValue: '取消' })}
          isDanger={true}
          onConfirm={handleConfirmDeleteStreak}
          onCancel={() => setStreakToDelete(null)}
        />
      )}
    </div>
  );
};
