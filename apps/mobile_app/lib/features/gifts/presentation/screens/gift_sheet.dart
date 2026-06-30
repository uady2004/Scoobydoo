import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/gifts/data/models/gift_model.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────────────────────

/// Shows the gift picker as a modal bottom sheet.
///
/// ```dart
/// showGiftSheet(context, targetUserId: 'abc123');
/// ```
Future<void> showGiftSheet(
  BuildContext context, {
  required String targetUserId,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: Colors.transparent,
    builder: (_) => _GiftSheet(targetUserId: targetUserId),
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// _GiftSheet
// ─────────────────────────────────────────────────────────────────────────────

class _GiftSheet extends ConsumerStatefulWidget {
  const _GiftSheet({required this.targetUserId});

  final String targetUserId;

  @override
  ConsumerState<_GiftSheet> createState() => _GiftSheetState();
}

class _GiftSheetState extends ConsumerState<_GiftSheet> {
  GiftModel? _selectedGift;
  int _selectedCount = 1;
  bool _isSending = false;

  static const _kRed = Color(0xFFEE1D52);
  static const _kGold = Color(0xFFFFD700);
  static const _quantities = [1, 10, 99];

  Future<void> _onSend() async {
    if (_selectedGift == null || _isSending) return;
    setState(() => _isSending = true);
    await Future.delayed(const Duration(seconds: 1));
    if (!mounted) return;
    setState(() {
      _isSending = false;
      _selectedGift = null;
    });
    Navigator.pop(context);
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Gift sent!'),
        behavior: SnackBarBehavior.floating,
        duration: Duration(seconds: 2),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.5,
      minChildSize: 0.4,
      maxChildSize: 0.7,
      snap: true,
      snapSizes: const [0.5, 0.7],
      builder: (context, scrollController) {
        return Container(
          decoration: const BoxDecoration(
            color: Colors.black,
            borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
          ),
          child: Column(
            children: [
              // ── Drag handle ───────────────────────────────────────────────
              Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.symmetric(vertical: 12),
                decoration: BoxDecoration(
                  color: Colors.white24,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),

              // ── Header ────────────────────────────────────────────────────
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 16),
                child: Row(
                  children: [
                    Text(
                      'Send a Gift',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    Spacer(),
                    Icon(Icons.toll_rounded, color: _kGold, size: 16),
                    SizedBox(width: 4),
                    Text(
                      '1,200 coins',
                      style: TextStyle(color: Colors.grey, fontSize: 13),
                    ),
                  ],
                ),
              ),

              const Divider(color: Colors.white12, height: 20),

              // ── Gift grid ─────────────────────────────────────────────────
              Expanded(
                child: GridView.count(
                  controller: scrollController,
                  crossAxisCount: 4,
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  padding: const EdgeInsets.symmetric(
                      horizontal: 12, vertical: 4),
                  mainAxisSpacing: 8,
                  crossAxisSpacing: 8,
                  childAspectRatio: 0.85,
                  children: GiftModel.defaults
                      .map(
                        (gift) => _GiftItem(
                          gift: gift,
                          isSelected: _selectedGift?.id == gift.id,
                          onTap: () =>
                              setState(() => _selectedGift = gift),
                        ),
                      )
                      .toList(),
                ),
              ),

              // ── Send bar (shown when a gift is selected) ──────────────────
              AnimatedSize(
                duration: const Duration(milliseconds: 200),
                curve: Curves.easeInOut,
                child: _selectedGift != null
                    ? _buildSendBar()
                    : const SizedBox.shrink(),
              ),

              SafeArea(
                top: false,
                child: SizedBox(
                  height: _selectedGift != null ? 0 : 8,
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildSendBar() {
    final gift = _selectedGift!;
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
      decoration: BoxDecoration(
        color: Colors.grey[900],
        border: const Border(
          top: BorderSide(color: Colors.white12),
        ),
      ),
      child: Row(
        children: [
          // Gift preview 48px
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: Colors.grey[800],
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: _kRed, width: 1.5),
            ),
            child: Center(
              child: Text(
                _giftEmoji(gift.animationKey),
                style: const TextStyle(fontSize: 24),
              ),
            ),
          ),
          const SizedBox(width: 10),

          // Name + coins column
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                gift.name,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.bold,
                  fontSize: 13,
                ),
              ),
              Row(
                children: [
                  const Icon(Icons.toll, color: _kGold, size: 12),
                  const SizedBox(width: 3),
                  Text(
                    '${gift.coinCost * _selectedCount} coins',
                    style: const TextStyle(
                        color: _kGold, fontSize: 11),
                  ),
                ],
              ),
            ],
          ),

          const Spacer(),

          // Quantity chips
          Row(
            children: _quantities.map((q) {
              final active = _selectedCount == q;
              return GestureDetector(
                onTap: () => setState(() => _selectedCount = q),
                child: Container(
                  margin: const EdgeInsets.only(right: 6),
                  padding: const EdgeInsets.symmetric(
                      horizontal: 10, vertical: 6),
                  decoration: BoxDecoration(
                    color: active ? _kRed : Colors.grey[800],
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    'x$q',
                    style: TextStyle(
                      color:
                          active ? Colors.white : Colors.grey,
                      fontWeight: FontWeight.bold,
                      fontSize: 12,
                    ),
                  ),
                ),
              );
            }).toList(),
          ),

          const SizedBox(width: 8),

          // Send button
          ElevatedButton(
            onPressed: _isSending ? null : _onSend,
            style: ElevatedButton.styleFrom(
              backgroundColor: _kRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(
                  horizontal: 16, vertical: 10),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              minimumSize: const Size(60, 36),
            ),
            child: _isSending
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
                  )
                : const Text(
                    'Send',
                    style: TextStyle(
                        fontWeight: FontWeight.bold,
                        fontSize: 14),
                  ),
          ),
        ],
      ),
    );
  }

  static String _giftEmoji(String key) {
    const map = <String, String>{
      'rose': '\u{1F339}',
      'tiktok': '\u{1F3B5}',
      'heart': '\u{2764}\u{FE0F}',
      'sunglasses': '\u{1F60E}',
      'perfume': '\u{1F9F4}',
      'mic': '\u{1F3A4}',
      'car': '\u{1F3CE}\u{FE0F}',
      'universe': '\u{1F30C}',
    };
    return map[key] ?? '\u{1F381}';
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// _GiftItem
// ─────────────────────────────────────────────────────────────────────────────

class _GiftItem extends StatelessWidget {
  const _GiftItem({
    required this.gift,
    required this.isSelected,
    required this.onTap,
  });

  final GiftModel gift;
  final bool isSelected;
  final VoidCallback onTap;

  static const _kRed = Color(0xFFEE1D52);
  static const _kGold = Color(0xFFFFD700);

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 60,
            height: 60,
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: isSelected
                    ? [
                        const Color(0xFF2A0A10),
                        const Color(0xFF1A0A20),
                      ]
                    : [
                        Colors.grey[850]!,
                        Colors.grey[900]!,
                      ],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: isSelected ? _kRed : Colors.transparent,
                width: 2,
              ),
            ),
            child: const Center(
              child: Icon(Icons.card_giftcard, color: _kGold, size: 28),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            gift.name,
            style: const TextStyle(color: Colors.white, fontSize: 11),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.center,
          ),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.toll, color: _kGold, size: 10),
              const SizedBox(width: 2),
              Text(
                '${gift.coinCost}',
                style: const TextStyle(color: Colors.grey, fontSize: 11),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy GiftSheet — kept for backward compatibility with livestream widgets
// that construct GiftSheet directly via GiftSheet.show(...).
// ─────────────────────────────────────────────────────────────────────────────

class GiftSheet extends ConsumerStatefulWidget {
  const GiftSheet({
    super.key,
    required this.streamId,
    required this.onSendGift,
    this.gifts,
  });

  final String streamId;
  final Future<void> Function(GiftModel gift, int quantity) onSendGift;
  final List<GiftModel>? gifts;

  static Future<void> show({
    required BuildContext context,
    required String streamId,
    required Future<void> Function(GiftModel gift, int quantity) onSendGift,
    List<GiftModel>? gifts,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => GiftSheet(
        streamId: streamId,
        onSendGift: onSendGift,
        gifts: gifts,
      ),
    );
  }

  @override
  ConsumerState<GiftSheet> createState() => _LegacyGiftSheetState();
}

class _LegacyGiftSheetState extends ConsumerState<GiftSheet> {
  GiftModel? _selected;
  int _quantity = 1;
  bool _sending = false;
  String _activeCategory = 'All';

  static const List<int> _quantities = [1, 10, 99];
  static const List<String> _categories = [
    'All',
    'basic',
    'premium',
    'luxury',
  ];

  List<GiftModel> get _gifts => widget.gifts ?? GiftModel.defaults;

  List<GiftModel> get _filtered => _activeCategory == 'All'
      ? _gifts
      : _gifts.where((g) => g.category == _activeCategory).toList();

  static const _kRed = Color(0xFFEE1D52);
  static const _kGold = Color(0xFFFFD700);

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.45,
      minChildSize: 0.35,
      maxChildSize: 0.70,
      snap: true,
      snapSizes: const [0.45, 0.70],
      builder: (context, scrollController) {
        return Container(
          decoration: const BoxDecoration(
            color: Color(0xFF1A1A1A),
            borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
          ),
          child: Column(
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: const Color(0xFF3A3A3A),
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 16),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Send a Gift',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 16,
                      ),
                    ),
                    Row(
                      children: [
                        Icon(Icons.monetization_on,
                            color: _kGold, size: 16),
                        SizedBox(width: 4),
                        Text(
                          '1,200',
                          style: TextStyle(
                            color: _kGold,
                            fontWeight: FontWeight.bold,
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 10),
              SizedBox(
                height: 32,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  itemCount: _categories.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (_, i) {
                    final cat = _categories[i];
                    final isActive = _activeCategory == cat;
                    return GestureDetector(
                      onTap: () => setState(() => _activeCategory = cat),
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 6),
                        decoration: BoxDecoration(
                          color: isActive
                              ? _kRed
                              : const Color(0xFF2A2A2A),
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Text(
                          cat == 'All'
                              ? 'All'
                              : cat[0].toUpperCase() +
                                  cat.substring(1),
                          style: TextStyle(
                            color: isActive
                                ? Colors.white
                                : const Color(0xFF888888),
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
              const SizedBox(height: 12),
              Expanded(
                child: GridView.builder(
                  controller: scrollController,
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  gridDelegate:
                      const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    mainAxisSpacing: 8,
                    crossAxisSpacing: 8,
                    childAspectRatio: 0.85,
                  ),
                  itemCount: _filtered.length,
                  itemBuilder: (_, i) {
                    final gift = _filtered[i];
                    final isSelected = _selected?.id == gift.id;
                    return _GiftItem(
                      gift: gift,
                      isSelected: isSelected,
                      onTap: () => setState(() => _selected = gift),
                    );
                  },
                ),
              ),
              if (_selected != null) _buildSendBar(),
              SizedBox(
                  height: MediaQuery.of(context).padding.bottom + 8),
            ],
          ),
        );
      },
    );
  }

  Widget _buildSendBar() {
    final gift = _selected!;
    final totalCost = gift.coinCost * _quantity;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
      decoration: BoxDecoration(
        color: const Color(0xFF121212),
        border: Border(
          top: BorderSide(
            color: Colors.white.withValues(alpha: 0.08),
          ),
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: const Color(0xFF2A2A2A),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: _kRed, width: 1.5),
            ),
            child: Center(
              child: Text(
                _giftEmoji(gift.animationKey),
                style: const TextStyle(fontSize: 22),
              ),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  gift.name,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                    fontSize: 13,
                  ),
                ),
                Row(
                  children: [
                    const Icon(Icons.monetization_on,
                        color: _kGold, size: 13),
                    const SizedBox(width: 3),
                    Text(
                      '$totalCost coins',
                      style: const TextStyle(
                          color: _kGold, fontSize: 12),
                    ),
                  ],
                ),
              ],
            ),
          ),
          Row(
            children: _quantities.map((q) {
              final active = _quantity == q;
              return GestureDetector(
                onTap: () => setState(() => _quantity = q),
                child: Container(
                  margin: const EdgeInsets.only(right: 6),
                  padding: const EdgeInsets.symmetric(
                      horizontal: 10, vertical: 6),
                  decoration: BoxDecoration(
                    color: active
                        ? _kRed
                        : const Color(0xFF2A2A2A),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    'x$q',
                    style: TextStyle(
                      color: active
                          ? Colors.white
                          : const Color(0xFF888888),
                      fontWeight: FontWeight.bold,
                      fontSize: 12,
                    ),
                  ),
                ),
              );
            }).toList(),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: _sending ? null : _send,
            child: Container(
              padding: const EdgeInsets.symmetric(
                  horizontal: 18, vertical: 10),
              decoration: BoxDecoration(
                gradient: !_sending
                    ? const LinearGradient(
                        colors: [Color(0xFFFF2D55), Color(0xFFFF6B8A)],
                      )
                    : null,
                color: _sending ? const Color(0xFF3A3A3A) : null,
                borderRadius: BorderRadius.circular(10),
              ),
              child: _sending
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Text(
                      'Send',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 14,
                      ),
                    ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _send() async {
    if (_selected == null || _sending) return;
    final gift = _selected!;
    final qty = _quantity;
    setState(() => _sending = true);
    try {
      await widget.onSendGift(gift, qty);
      if (mounted) {
        setState(() {
          _selected = null;
          _quantity = 1;
        });
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  static String _giftEmoji(String key) {
    const map = <String, String>{
      'rose': '\u{1F339}',
      'tiktok': '\u{1F3B5}',
      'heart': '\u{2764}\u{FE0F}',
      'sunglasses': '\u{1F60E}',
      'perfume': '\u{1F9F4}',
      'mic': '\u{1F3A4}',
      'car': '\u{1F3CE}\u{FE0F}',
      'universe': '\u{1F30C}',
      'kiss': '\u{1F48B}',
      'thumbs_up': '\u{1F44D}',
      'ice_cream': '\u{1F366}',
      'galaxy': '\u{1F30C}',
      'crown': '\u{1F451}',
      'rocket': '\u{1F680}',
      'diamond_ring': '\u{1F48D}',
      'yacht': '\u{1F6A2}',
      'castle': '\u{1F3F0}',
    };
    return map[key] ?? '\u{1F381}';
  }
}
