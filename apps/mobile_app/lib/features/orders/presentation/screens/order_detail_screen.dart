import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../ecommerce/domain/entities/order_entity.dart';
import '../../../ecommerce/presentation/providers/ecommerce_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// OrderDetailScreen
// ─────────────────────────────────────────────────────────────────────────────

class OrderDetailScreen extends ConsumerWidget {
  const OrderDetailScreen({super.key, required this.orderId});
  final String orderId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncOrder = ref.watch(orderDetailProvider(orderId));

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Colors.white),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text(
          'Order Details',
          style: TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.w700),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.share_outlined, color: Colors.white),
            onPressed: () {
              final order = asyncOrder.valueOrNull;
              if (order == null) return;
              Clipboard.setData(ClipboardData(
                text: 'Order #${order.id.substring(0, 8).toUpperCase()} '
                    '— \$${order.total.toStringAsFixed(2)}',
              ));
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  backgroundColor: Color(0xFF1A1A1A),
                  content: Text('Order info copied to clipboard.',
                      style: TextStyle(color: Colors.white)),
                  duration: Duration(seconds: 2),
                ),
              );
            },
          ),
        ],
      ),
      body: asyncOrder.when(
        loading: () => const Center(
          child: CircularProgressIndicator(
              color: Color(0xFFFF2D55), strokeWidth: 2),
        ),
        error: (err, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(Icons.error_outline,
                  color: Color(0xFFFF2D55), size: 48),
              const SizedBox(height: 12),
              Text(err.toString(),
                  style: const TextStyle(color: Colors.white70)),
              TextButton(
                onPressed: () =>
                    ref.invalidate(orderDetailProvider(orderId)),
                child: const Text('Retry',
                    style: TextStyle(color: Color(0xFFFF2D55))),
              ),
            ],
          ),
        ),
        data: (order) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _TrackingTimeline(order: order),
            const SizedBox(height: 24),
            _SectionCard(
              title: 'Items (${order.itemCount})',
              child: Column(
                children: order.items.map((item) {
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: _OrderItemRow(item: item),
                  );
                }).toList(),
              ),
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Delivery Address',
              child: _AddressBlock(info: order.buyerInfo),
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Payment',
              child: Row(
                children: [
                  const Icon(Icons.credit_card,
                      color: Color(0xFF888888), size: 20),
                  const SizedBox(width: 10),
                  Text(
                    order.paymentMethod == 'coins'
                        ? 'Paid with Coins'
                        : 'Card payment',
                    style: const TextStyle(
                        color: Colors.white, fontSize: 14),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Order Summary',
              child: Column(
                children: [
                  _SummaryRow(
                      label: 'Subtotal',
                      value: '\$${order.subtotal.toStringAsFixed(2)}'),
                  const SizedBox(height: 6),
                  _SummaryRow(
                    label: 'Shipping',
                    value: order.shippingFee == 0
                        ? 'Free'
                        : '\$${order.shippingFee.toStringAsFixed(2)}',
                  ),
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 10),
                    child: Divider(
                        color: Color(0xFF2A2A2A),
                        thickness: 1,
                        height: 1),
                  ),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text('Total',
                          style: TextStyle(
                              color: Colors.white,
                              fontSize: 15,
                              fontWeight: FontWeight.w700)),
                      Text('\$${order.total.toStringAsFixed(2)}',
                          style: const TextStyle(
                              color: Color(0xFFFF2D55),
                              fontSize: 18,
                              fontWeight: FontWeight.w800)),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () {},
                icon: const Icon(Icons.support_agent_outlined, size: 18),
                label: const Text('Contact Support'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: Colors.white,
                  side: const BorderSide(color: Color(0xFF333333)),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                ),
              ),
            ),
            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Tracking timeline
// ─────────────────────────────────────────────────────────────────────────────

class _TrackingTimeline extends StatelessWidget {
  const _TrackingTimeline({required this.order});
  final OrderEntity order;

  static const _steps = [
    _StepDef(icon: Icons.receipt_outlined, label: 'Order Placed'),
    _StepDef(icon: Icons.verified_outlined, label: 'Payment Confirmed'),
    _StepDef(icon: Icons.inventory_2_outlined, label: 'Processing'),
    _StepDef(icon: Icons.local_shipping_outlined, label: 'Shipped'),
    _StepDef(icon: Icons.delivery_dining_outlined, label: 'Out for Delivery'),
    _StepDef(icon: Icons.home_outlined, label: 'Delivered'),
  ];

  int get _currentStepIndex {
    switch (order.status) {
      case OrderStatus.pending:
        return 0;
      case OrderStatus.processing:
        return 2;
      case OrderStatus.shipped:
        return 3;
      case OrderStatus.delivered:
        return 5;
      case OrderStatus.cancelled:
      case OrderStatus.returned:
        return -1;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (order.status == OrderStatus.cancelled ||
        order.status == OrderStatus.returned) {
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: const Color(0xFF2A0A0A),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: const Color(0xFF5A1A1A)),
        ),
        child: Row(
          children: [
            const Icon(Icons.cancel_outlined,
                color: Color(0xFFFF5252), size: 24),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    order.status == OrderStatus.cancelled
                        ? 'Order Cancelled'
                        : 'Order Returned',
                    style: const TextStyle(
                        color: Color(0xFFFF5252),
                        fontSize: 15,
                        fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Order #${order.id.substring(0, 8).toUpperCase()}',
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 13),
                  ),
                ],
              ),
            ),
          ],
        ),
      );
    }

    final current = _currentStepIndex;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF111111),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Tracking',
            style: TextStyle(
                color: Colors.white,
                fontSize: 15,
                fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 16),
          ...List.generate(_steps.length, (i) {
            final isShippedStep = i == 3;
            return _TimelineRow(
              stepDef: _steps[i],
              isDone: i < current,
              isCurrent: i == current,
              isLast: i == _steps.length - 1,
              extraWidget: (isShippedStep &&
                      order.trackingNumber != null &&
                      (i <= current))
                  ? _TrackingInfo(
                      trackingNumber: order.trackingNumber!,
                      courier: order.courierName ?? 'Standard Shipping',
                    )
                  : null,
            );
          }),
        ],
      ),
    );
  }
}

class _StepDef {
  const _StepDef({required this.icon, required this.label});
  final IconData icon;
  final String label;
}

class _TimelineRow extends StatefulWidget {
  const _TimelineRow({
    required this.stepDef,
    required this.isDone,
    required this.isCurrent,
    required this.isLast,
    this.extraWidget,
  });

  final _StepDef stepDef;
  final bool isDone;
  final bool isCurrent;
  final bool isLast;
  final Widget? extraWidget;

  @override
  State<_TimelineRow> createState() => _TimelineRowState();
}

class _TimelineRowState extends State<_TimelineRow>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pulse;

  @override
  void initState() {
    super.initState();
    _pulse = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    );
    if (widget.isCurrent) _pulse.repeat(reverse: true);
  }

  @override
  void dispose() {
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final Color nodeColor;
    if (widget.isDone) {
      nodeColor = const Color(0xFF4CAF50);
    } else if (widget.isCurrent) {
      nodeColor = const Color(0xFFFF2D55);
    } else {
      nodeColor = const Color(0xFF333333);
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 32,
          child: Column(
            children: [
              widget.isCurrent
                  ? AnimatedBuilder(
                      animation: _pulse,
                      builder: (_, __) => Container(
                        width: 28,
                        height: 28,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: Color.fromARGB(
                            (80 + (_pulse.value * 100).toInt()),
                            0xFF,
                            0x2D,
                            0x55,
                          ),
                        ),
                        child: const Icon(Icons.circle,
                            color: Color(0xFFFF2D55), size: 12),
                      ),
                    )
                  : Container(
                      width: 28,
                      height: 28,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: widget.isDone
                            ? const Color(0xFF0D2E1A)
                            : const Color(0xFF1A1A1A),
                        border:
                            Border.all(color: nodeColor, width: 1.5),
                      ),
                      child: Icon(
                        widget.isDone
                            ? Icons.check
                            : widget.stepDef.icon,
                        color: nodeColor,
                        size: 14,
                      ),
                    ),
              if (!widget.isLast)
                Container(
                  width: 2,
                  height: 40,
                  color: widget.isDone
                      ? const Color(0xFF4CAF50)
                      : const Color(0xFF2A2A2A),
                ),
            ],
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Padding(
            padding: EdgeInsets.only(
              top: 4,
              bottom: widget.isLast ? 0 : 28,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.stepDef.label,
                  style: TextStyle(
                    color: widget.isDone || widget.isCurrent
                        ? Colors.white
                        : const Color(0xFF555555),
                    fontSize: 14,
                    fontWeight: widget.isCurrent
                        ? FontWeight.w700
                        : FontWeight.w400,
                  ),
                ),
                if (widget.extraWidget != null) ...[
                  const SizedBox(height: 8),
                  widget.extraWidget!,
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _TrackingInfo extends StatelessWidget {
  const _TrackingInfo(
      {required this.trackingNumber, required this.courier});
  final String trackingNumber;
  final String courier;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: const Color(0xFF1A1A1A),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(courier,
              style: const TextStyle(
                  color: Color(0xFF888888), fontSize: 12)),
          const SizedBox(height: 4),
          Row(
            children: [
              Expanded(
                child: Text(
                  trackingNumber,
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 0.5),
                ),
              ),
              GestureDetector(
                onTap: () => Clipboard.setData(
                    ClipboardData(text: trackingNumber)),
                child: const Icon(Icons.copy_outlined,
                    color: Color(0xFF888888), size: 16),
              ),
            ],
          ),
          const SizedBox(height: 8),
          GestureDetector(
            onTap: () {},
            child: const Text(
              'Track Package →',
              style: TextStyle(
                  color: Color(0xFFFF2D55),
                  fontSize: 12,
                  fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Section card
// ─────────────────────────────────────────────────────────────────────────────

class _SectionCard extends StatelessWidget {
  const _SectionCard({required this.title, required this.child});
  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF111111),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title,
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w700)),
          const SizedBox(height: 12),
          child,
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Order item row
// ─────────────────────────────────────────────────────────────────────────────

class _OrderItemRow extends StatelessWidget {
  const _OrderItemRow({required this.item});
  final OrderItemEntity item;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: SizedBox(
            width: 56,
            height: 56,
            child: item.product.thumbnailUrl.isNotEmpty
                ? Image.network(
                    item.product.thumbnailUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) =>
                        Container(color: const Color(0xFF2A2A2A)),
                    loadingBuilder: (_, child, progress) {
                      if (progress == null) return child;
                      return Container(color: const Color(0xFF2A2A2A));
                    },
                  )
                : Container(color: const Color(0xFF2A2A2A)),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                item.product.name,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w500),
              ),
              if (item.variant != null)
                Text(item.variant!.name,
                    style: const TextStyle(
                        color: Color(0xFF888888), fontSize: 12)),
            ],
          ),
        ),
        const SizedBox(width: 8),
        Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text('x${item.qty}',
                style: const TextStyle(
                    color: Color(0xFF888888), fontSize: 12)),
            Text('\$${item.subtotal.toStringAsFixed(2)}',
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w600)),
          ],
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Address block
// ─────────────────────────────────────────────────────────────────────────────

class _AddressBlock extends StatelessWidget {
  const _AddressBlock({required this.info});
  final BuyerInfoEntity info;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(info.name,
            style: const TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w600)),
        const SizedBox(height: 6),
        Text(info.fullAddress,
            style: const TextStyle(
                color: Color(0xFF888888),
                fontSize: 13,
                height: 1.5)),
        const SizedBox(height: 4),
        Text(info.phone,
            style: const TextStyle(
                color: Color(0xFF888888), fontSize: 13)),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Summary row
// ─────────────────────────────────────────────────────────────────────────────

class _SummaryRow extends StatelessWidget {
  const _SummaryRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label,
            style: const TextStyle(
                color: Color(0xFF888888), fontSize: 14)),
        Text(value,
            style: const TextStyle(
                color: Colors.white, fontSize: 14)),
      ],
    );
  }
}
