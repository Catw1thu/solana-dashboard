use super::model::{InvocationSource, PumpfunInvocation};
use crate::{
    transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    },
};

#[cfg(test)]
use super::model::InstructionInput;
#[cfg(test)]
use crate::pumpfun::instruction::parse_instruction;

#[cfg(test)]
fn build_input_from_outer(ix: &OuterInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

#[cfg(test)]
fn build_input_from_inner(ix: &InnerInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

#[cfg(test)]
pub fn extract_invocations(view: &TransactionView) -> Vec<PumpfunInvocation> {
    let mut pending_inner_groups = view
        .inner_instruction_groups
        .iter()
        .map(|group| (group.outer_instruction_index as usize, group))
        .collect::<std::collections::BTreeMap<_, _>>();

    let mut invocations = Vec::new();

    for (outer_index, outer_ix) in view.outer_instructions.iter().enumerate() {
        if let Some(instruction) = parse_instruction(&build_input_from_outer(outer_ix)) {
            invocations.push(PumpfunInvocation {
                source: InvocationSource::Outer { outer_index },
                instruction,
            });
        }

        if let Some(group) = pending_inner_groups.remove(&outer_index) {
            collect_inner_group_invocations(group, outer_index, &mut invocations);
        }
    }

    for (outer_index, group) in pending_inner_groups {
        collect_inner_group_invocations(group, outer_index, &mut invocations);
    }

    invocations
}

#[cfg(test)]
fn collect_inner_group_invocations(
    group: &InnerInstructionGroup,
    outer_index: usize,
    invocations: &mut Vec<PumpfunInvocation>,
) {
    for (inner_index, ix) in group.instructions.iter().enumerate() {
        let input = build_input_from_inner(ix);

        let Some(instruction) = parse_instruction(&input) else {
            continue;
        };

        invocations.push(PumpfunInvocation {
            source: InvocationSource::Inner {
                outer_index,
                inner_index,
            },
            instruction,
        });
    }
}

#[cfg(test)]
mod tests {
    use super::extract_invocations;
    use crate::{
        pumpfun::{
            PUMPFUN_PROGRAM_ID,
            discriminators::{BUY_IX_DISC, SELL_IX_DISC},
            model::{InvocationSource, PumpfunInstruction},
        },
        transaction_view::{
            InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
        },
    };
    use base64::{Engine as _, engine::general_purpose::STANDARD};

    #[test]
    fn extract_invocations_follows_execution_order() {
        let view = TransactionView {
            slot: 0,
            signature: "sig".to_string(),
            tx_index: 0,
            is_vote: false,
            account_keys: vec![],
            loaded_writable_addresses: vec![],
            loaded_readonly_addresses: vec![],
            all_accounts: vec![],
            outer_instructions: vec![other_outer_ix(), pumpfun_sell_outer_ix()],
            inner_instruction_groups: vec![InnerInstructionGroup {
                outer_instruction_index: 0,
                instructions: vec![other_inner_ix(), pumpfun_buy_inner_ix()],
            }],
            log_messages: vec![],
        };

        let invocations = extract_invocations(&view);

        assert_eq!(invocations.len(), 2);
        assert!(matches!(
            invocations[0].source,
            InvocationSource::Inner {
                outer_index: 0,
                inner_index: 1,
            }
        ));
        assert!(matches!(
            invocations[0].instruction,
            PumpfunInstruction::Buy(_)
        ));
        assert!(matches!(
            invocations[1].source,
            InvocationSource::Outer { outer_index: 1 }
        ));
        assert!(matches!(
            invocations[1].instruction,
            PumpfunInstruction::Sell(_)
        ));
    }

    fn other_outer_ix() -> OuterInstructionView {
        OuterInstructionView {
            program_id_index: 0,
            program_id: "other-program".to_string(),
            account_indices: vec![],
            account_pubkeys: vec![],
            data_len: 0,
            data_prefix: vec![],
            data_base64: STANDARD.encode([]),
        }
    }

    fn pumpfun_sell_outer_ix() -> OuterInstructionView {
        OuterInstructionView {
            program_id_index: 0,
            program_id: PUMPFUN_PROGRAM_ID.to_string(),
            account_indices: (0..14).collect(),
            account_pubkeys: account_pubkeys(14),
            data_len: 24,
            data_prefix: sell_ix_data()[0..16].to_vec(),
            data_base64: STANDARD.encode(sell_ix_data()),
        }
    }

    fn other_inner_ix() -> InnerInstructionView {
        InnerInstructionView {
            program_id_index: 0,
            program_id: "other-program".to_string(),
            account_indices: vec![],
            account_pubkeys: vec![],
            data_len: 0,
            data_prefix: vec![],
            data_base64: STANDARD.encode([]),
            stack_height: Some(2),
        }
    }

    fn pumpfun_buy_inner_ix() -> InnerInstructionView {
        InnerInstructionView {
            program_id_index: 0,
            program_id: PUMPFUN_PROGRAM_ID.to_string(),
            account_indices: (0..16).collect(),
            account_pubkeys: account_pubkeys(16),
            data_len: 24,
            data_prefix: buy_ix_data()[0..16].to_vec(),
            data_base64: STANDARD.encode(buy_ix_data()),
            stack_height: Some(2),
        }
    }

    fn account_pubkeys(count: usize) -> Vec<String> {
        (0..count).map(|index| format!("account-{index}")).collect()
    }

    fn buy_ix_data() -> Vec<u8> {
        let mut data = Vec::from(BUY_IX_DISC);
        data.extend_from_slice(&1u64.to_le_bytes());
        data.extend_from_slice(&2u64.to_le_bytes());
        data
    }

    fn sell_ix_data() -> Vec<u8> {
        let mut data = Vec::from(SELL_IX_DISC);
        data.extend_from_slice(&3u64.to_le_bytes());
        data.extend_from_slice(&4u64.to_le_bytes());
        data
    }
}
