import 'dart:async';

import 'package:flutter/material.dart';
import '../models/livestream_model.dart';

/// Split-screen PK battle score overlay shown at the top of the stream.
class PKBattleOverlay extends StatefulWidget {
  const PKBattleOverlay({
    super.key,
    required this.battle,
  });

  final PKBattle battle;

  @override
  State<PKBattleOverlay> createState() => _PKBattleOverlayState();
}

class _PKBattleOverlayState extends State<PKBattleOverlay> {
  late int _secondsLeft;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _secondsLeft = _computeSecondsLeft();
    if (widget.battle.status == BattleStatus.active) {
      _startTimer();
    }
  }

  @override
  void didUpdateWidget(PKBattleOverlay old) {
    super.didUpdateWidget(old);
    if (widget.battle.status == BattleStatus.active &&
        old.battle.status != BattleStatus.active) {
      _secondsLeft = _computeSecondsLeft();
      _startTimer();
    }
    if (widget.battle.status == BattleStatus.ended) {
      _timer?.cancel();
    }
  }

  int _computeSecondsLeft() {
    if (widget.battle.startedAt == null) return widget.battle.durationSecs;
    final elapsed =
        DateTime.now().difference(widget.battle.startedAt!).inSeconds;
    return (widget.battle.durationSecs - elapsed).clamp(0, widget.battle.durationSecs);
  }

  void _startTimer() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() {
        if (_secondsLeft > 0) {
          _secondsLeft--;
        } else {
          _timer?.cancel();
        }
      });
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final totalScore =
        widget.battle.initiatorScore + widget.battle.targetScore;
    final double initiatorFraction =
        totalScore == 0 ? 0.5 : widget.battle.initiatorScore / totalScore;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.55),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildNameRow(),
          const SizedBox(height: 6),
          _buildProgressBar(initiatorFraction),
          const SizedBox(height: 4),
          _buildScoreRow(),
          if (widget.battle.status == BattleStatus.ended)
            _buildWinnerBanner(),
        ],
      ),
    );
  }

  Widget _buildNameRow() {
    return Row(
      children: [
        Expanded(
          child: Text(
            widget.battle.initiatorName,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.bold,
              fontSize: 13,
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ),
        // Countdown timer
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          decoration: BoxDecoration(
            color: _secondsLeft <= 10
                ? Colors.red.withValues(alpha: 0.8)
                : const Color(0xFFFF2D55).withValues(alpha: 0.8),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            '${_secondsLeft}s',
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.bold,
              fontSize: 12,
            ),
          ),
        ),
        Expanded(
          child: Text(
            widget.battle.targetName,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.bold,
              fontSize: 13,
            ),
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.end,
          ),
        ),
      ],
    );
  }

  Widget _buildProgressBar(double initiatorFraction) {
    return Stack(
      alignment: Alignment.center,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: initiatorFraction,
            minHeight: 8,
            backgroundColor: const Color(0xFF00C6FF),
            valueColor:
                const AlwaysStoppedAnimation<Color>(Color(0xFFFF2D55)),
          ),
        ),
        const Positioned(
          child: Icon(Icons.flash_on, color: Colors.white, size: 14),
        ),
      ],
    );
  }

  Widget _buildScoreRow() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Row(
          children: [
            const Icon(Icons.monetization_on, color: Colors.amber, size: 12),
            const SizedBox(width: 2),
            Text(
              '${widget.battle.initiatorScore}',
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.bold),
            ),
          ],
        ),
        Row(
          children: [
            Text(
              '${widget.battle.targetScore}',
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.bold),
            ),
            const SizedBox(width: 2),
            const Icon(Icons.monetization_on, color: Colors.amber, size: 12),
          ],
        ),
      ],
    );
  }

  Widget _buildWinnerBanner() {
    final isInitiatorWinner =
        widget.battle.winnerId == widget.battle.initiatorId;
    final winnerName = isInitiatorWinner
        ? widget.battle.initiatorName
        : widget.battle.targetName;
    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Text(
        '$winnerName wins! 🎉',
        style: const TextStyle(
          color: Colors.amber,
          fontWeight: FontWeight.bold,
          fontSize: 13,
        ),
        textAlign: TextAlign.center,
      ),
    );
  }
}
