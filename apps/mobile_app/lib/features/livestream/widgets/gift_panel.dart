import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../models/livestream_model.dart';

/// Bottom sheet panel showing the gift catalog so the viewer can send gifts.
class GiftPanel extends StatefulWidget {
  const GiftPanel({
    super.key,
    required this.catalog,
    required this.coinBalance,
    required this.onSendGift,
  });

  final List<GiftType> catalog;
  final int coinBalance;
  final Future<void> Function(String giftTypeId, int quantity) onSendGift;

  @override
  State<GiftPanel> createState() => _GiftPanelState();
}

class _GiftPanelState extends State<GiftPanel> {
  GiftType? _selected;
  int _quantity = 1;
  bool _sending = false;

  bool get _canAfford =>
      _selected != null &&
      widget.coinBalance >= _selected!.coinPrice * _quantity;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 340,
      decoration: const BoxDecoration(
        color: Color(0xFF1A1A1A),
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: Column(
        children: [
          _buildHandle(),
          _buildCoinBalance(),
          Expanded(child: _buildGrid()),
          _buildSendRow(),
        ],
      ),
    );
  }

  Widget _buildHandle() {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Container(
        width: 36,
        height: 4,
        decoration: BoxDecoration(
          color: Colors.white.withValues(alpha: 0.3),
          borderRadius: BorderRadius.circular(2),
        ),
      ),
    );
  }

  Widget _buildCoinBalance() {
    return Padding(
      padding: const EdgeInsets.only(left: 16, right: 16, bottom: 8),
      child: Row(
        children: [
          const Icon(Icons.monetization_on, color: Colors.amber, size: 18),
          const SizedBox(width: 6),
          Text(
            '${widget.coinBalance} coins',
            style: const TextStyle(color: Colors.white, fontSize: 13),
          ),
        ],
      ),
    );
  }

  Widget _buildGrid() {
    final groups = <String, List<GiftType>>{};
    for (final g in widget.catalog) {
      groups.putIfAbsent(g.category, () => []).add(g);
    }

    final categories = groups.keys.toList();

    return DefaultTabController(
      length: categories.length,
      child: Column(
        children: [
          TabBar(
            isScrollable: true,
            labelColor: const Color(0xFFFF2D55),
            unselectedLabelColor: Colors.white54,
            indicatorColor: const Color(0xFFFF2D55),
            tabs: categories
                .map((c) => Tab(
                      text: c[0].toUpperCase() + c.substring(1),
                    ))
                .toList(),
          ),
          Expanded(
            child: TabBarView(
              children: categories.map((cat) {
                final items = groups[cat]!;
                return GridView.builder(
                  padding: const EdgeInsets.all(8),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    mainAxisSpacing: 8,
                    crossAxisSpacing: 8,
                    childAspectRatio: 0.75,
                  ),
                  itemCount: items.length,
                  itemBuilder: (_, i) => _GiftTile(
                    gift: items[i],
                    selected: _selected?.id == items[i].id,
                    onTap: () => setState(() {
                      _selected = items[i];
                      _quantity = 1;
                    }),
                  ),
                );
              }).toList(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSendRow() {
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        bottom: MediaQuery.of(context).padding.bottom + 8,
        top: 8,
      ),
      child: Row(
        children: [
          // Quantity stepper
          if (_selected != null) ...[
            _QuantityStepper(
              quantity: _quantity,
              onDecrement: _quantity > 1
                  ? () => setState(() => _quantity--)
                  : null,
              onIncrement: () => setState(() => _quantity++),
            ),
            const SizedBox(width: 12),
          ],
          Expanded(
            child: SizedBox(
              height: 44,
              child: ElevatedButton(
                onPressed: (_selected != null && _canAfford && !_sending)
                    ? _sendGift
                    : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF2D55),
                  disabledBackgroundColor: Colors.grey.shade800,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(22),
                  ),
                ),
                child: _sending
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : Text(
                        _selected == null
                            ? 'Select a Gift'
                            : !_canAfford
                                ? 'Not Enough Coins'
                                : 'Send  ${_selected!.coinPrice * _quantity} 🪙',
                        style: const TextStyle(
                            color: Colors.white, fontWeight: FontWeight.bold),
                      ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _sendGift() async {
    if (_selected == null) return;
    setState(() => _sending = true);
    try {
      await widget.onSendGift(_selected!.id, _quantity);
      if (mounted) Navigator.pop(context);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }
}

class _GiftTile extends StatelessWidget {
  const _GiftTile({
    required this.gift,
    required this.selected,
    required this.onTap,
  });

  final GiftType gift;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        decoration: BoxDecoration(
          color: selected
              ? const Color(0xFFFF2D55).withValues(alpha: 0.2)
              : Colors.white.withValues(alpha: 0.05),
          borderRadius: BorderRadius.circular(10),
          border: Border.all(
            color: selected ? const Color(0xFFFF2D55) : Colors.transparent,
            width: 1.5,
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Expanded(
              child: Padding(
                padding: const EdgeInsets.all(6),
                child: CachedNetworkImage(
                  imageUrl: gift.iconUrl,
                  fit: BoxFit.contain,
                  errorWidget: (_, __, ___) =>
                      const Icon(Icons.card_giftcard, color: Colors.white54),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 4),
              child: Text(
                gift.name,
                style: const TextStyle(color: Colors.white, fontSize: 10),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.center,
              ),
            ),
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const Icon(Icons.monetization_on,
                      color: Colors.amber, size: 10),
                  const SizedBox(width: 2),
                  Text(
                    '${gift.coinPrice}',
                    style: const TextStyle(color: Colors.amber, fontSize: 10),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _QuantityStepper extends StatelessWidget {
  const _QuantityStepper({
    required this.quantity,
    required this.onIncrement,
    this.onDecrement,
  });

  final int quantity;
  final VoidCallback onIncrement;
  final VoidCallback? onDecrement;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _StepButton(
          icon: Icons.remove,
          onTap: onDecrement,
        ),
        const SizedBox(width: 8),
        Text(
          '$quantity',
          style: const TextStyle(
              color: Colors.white, fontWeight: FontWeight.bold, fontSize: 15),
        ),
        const SizedBox(width: 8),
        _StepButton(
          icon: Icons.add,
          onTap: onIncrement,
        ),
      ],
    );
  }
}

class _StepButton extends StatelessWidget {
  const _StepButton({required this.icon, this.onTap});

  final IconData icon;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 28,
        height: 28,
        decoration: BoxDecoration(
          color: onTap != null
              ? Colors.white.withValues(alpha: 0.15)
              : Colors.white.withValues(alpha: 0.05),
          shape: BoxShape.circle,
        ),
        child: Icon(
          icon,
          color: onTap != null ? Colors.white : Colors.white30,
          size: 16,
        ),
      ),
    );
  }
}
